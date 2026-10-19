//go:build linux

package container

// ExecProfile — снимок политики безопасности контейнера, который процесс
// инициализации оставляет ВНУТРИ контейнера.
//
// Зачем: helper-процесс запускается уже в namespace'ах контейнера и не видит
// state-каталог хоста, откуда он взял бы capability-набор. Раньше exec-путь
// вообще ничего не применял: команда получала полный набор capability демона
// (CAP_SYS_ADMIN + CAP_MKNOD) и uid 0, а seccomp-фильтр ставился только на
// init-процессе контейнера. Этого хватало для выхода наружу через
// mknod + mount блочного устройства хоста.
//
// Профиль пишется один раз при старте контейнера и читается helper'ом перед
// exec'ом целевой команды.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	// ExecHelperPath — путь к бинарнику-хелперу внутри контейнера.
	// Заполняется bind-mount'ом самого cardinal (см. bindExecHelper).
	ExecHelperPath = "/cardinal/.cardinal"

	// ExecProfilePath — профиль безопасности внутри контейнера.
	ExecProfilePath = "/cardinal/.exec-profile.json"

	// ExecHelperDir — каталог с утилитами cardinal внутри контейнера.
	ExecHelperDir = "/cardinal"
)

// ExecProfile описывает, кем и с какими правами должен выполняться exec
// внутри контейнера.
type ExecProfile struct {
	// UID/GID — пользователь контейнера (HasUser=false => остаёмся uid 0,
	// но capability-набор всё равно ограничивается).
	UID     int  `json:"uid"`
	GID     int  `json:"gid"`
	HasUser bool `json:"has_user"`

	// Capabilities — разрешённые capability (имена, без префикса CAP_).
	// Уже разрешённый набор: тот же, что применяет init-процесс.
	Capabilities []string `json:"capabilities"`

	// SeccompPath — путь к JSON-профилю seccomp ВНУТРИ контейнера
	// (пусто => используется профиль по умолчанию).
	SeccompPath string `json:"seccomp_path,omitempty"`

	// WorkingDir — рабочий каталог для exec.
	WorkingDir string `json:"working_dir,omitempty"`

	// Env — окружение контейнера для exec-процесса.
	Env []string `json:"env,omitempty"`
}

// buildExecProfile собирает профиль из состояния контейнера.
func buildExecProfile(c *Container) (*ExecProfile, error) {
	keep, err := requestedCapabilities(c)
	if err != nil {
		return nil, err
	}
	caps := make([]string, 0, len(keep))
	for name := range keep {
		caps = append(caps, name)
	}

	p := &ExecProfile{
		Capabilities: caps,
		WorkingDir:   c.WorkingDir,
		Env:          append([]string(nil), c.Env...),
	}

	// Пользователь контейнера. Пустой User => uid 0 (как сейчас), но
	// capability-набор всё равно применяется — именно он закрывал выход.
	if c.User != "" {
		parts := strings.Split(c.User, ":")
		uid, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parse container user %q: %w", c.User, err)
		}
		p.HasUser = true
		p.UID = uid
		if len(parts) == 2 {
			if gid, err := strconv.Atoi(parts[1]); err == nil {
				p.GID = gid
			}
		}
		if p.GID == 0 {
			p.GID = uid
		}
	}
	return p, nil
}

// writeExecProfile сохраняет профиль в каталог утилит внутри контейнера.
func writeExecProfile(c *Container, dir string) error {
	p, err := buildExecProfile(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(ExecProfilePath)), data, 0600); err != nil {
		return fmt.Errorf("write exec profile: %w", err)
	}
	return nil
}

// stageExecProfile готовит профиль безопасности ВНУТРИ rootfs контейнера.
//
// Вызывается ДО pivot_root, пока корень хоста ещё виден: пользовательский
// профиль seccomp лежит по хостовому пути, который после pivot_root
// становится недоступен, поэтому его нужно прочитать и положить рядом.
func stageExecProfile(c *Container, merged string) error {
	dir := filepath.Join(merged, strings.TrimPrefix(ExecHelperDir, "/"))
	if err := writeExecProfile(c, dir); err != nil {
		return err
	}

	// Пользовательский профиль seccomp: копируем внутрь контейнера, путь на
	// хосте оттуда не виден.
	if c.SeccompProfile != "" {
		src := c.SeccompProfile
		if !filepath.IsAbs(src) {
			src = filepath.Join(dir, src)
		}
		profile, err := LoadSeccompProfile(src)
		if err != nil {
			return fmt.Errorf("load seccomp profile: %w", err)
		}
		out := filepath.Join(dir, ".exec-seccomp.json")
		if err := SaveSeccompProfile(profile, out); err != nil {
			return fmt.Errorf("save seccomp profile: %w", err)
		}
		// Профиль уже разобран — сохраняем путь ВНУТРИ контейнера.
		p, err := loadExecProfile(filepath.Join(dir, filepath.Base(ExecProfilePath)))
		if err != nil {
			return err
		}
		p.SeccompPath = filepath.Join(ExecHelperDir, ".exec-seccomp.json")
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(ExecProfilePath)), data, 0600); err != nil {
			return fmt.Errorf("write exec profile: %w", err)
		}
	}
	return nil
}

// loadExecProfile читает профиль из контейнера.
func loadExecProfile(path string) (*ExecProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read exec profile: %w", err)
	}
	var p ExecProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse exec profile: %w", err)
	}
	return &p, nil
}

// bindExecHelper делает бинарник cardinal доступным внутри контейнера.
//
// Вызывается ДО pivot_root, когда корень хоста ещё виден: исходник —
// исполняемый файл самого cardinal, приёмник — путь внутри rootfs контейнера.
// Bind-mount одного файла не съедает дисковое пространство (в отличие от
// копирования бинарника в каждый контейнер) и не переживает pivot_root.
//
// Если монтирование не удалось (нет прав, занят путь) — это не фатально:
// init продолжает работать, а exec-путь падает с понятной ошибкой вместо
// того, чтобы работать без защиты.
func bindExecHelper(merged string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate cardinal binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	if _, err := os.Stat(self); err != nil {
		return fmt.Errorf("stat cardinal binary: %w", err)
	}

	dir := filepath.Join(merged, strings.TrimPrefix(ExecHelperDir, "/"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	target := filepath.Join(dir, ".cardinal")
	// Приёмник должен существовать, иначе bind-mount падает с ENOENT.
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	f.Close()

	if err := bindMountFile(self, target); err != nil {
		return fmt.Errorf("bind mount helper: %w", err)
	}
	return nil
}

// bindMountFile монтирует один файл в другой путь (bind mount).
func bindMountFile(source, target string) error {
	return syscall.Mount(source, target, "", syscall.MS_BIND, "")
}

// ensureExecHelper проверяет, что helper доступен внутри контейнера.
//
// Без helper'а команда выполнилась бы вообще без ограничений (как раньше),
// поэтому падаем явно, а не «как раньше». Контейнеры, запущенные до
// появления helper'а, нужно перезапустить.
func (c *Container) ensureExecHelper() error {
	_, _, merged := c.OverlayDirs()
	hostPath := filepath.Join(merged, strings.TrimPrefix(ExecHelperPath, "/"))
	if _, err := os.Stat(hostPath); err != nil {
		return fmt.Errorf("exec helper %s is not provisioned in container %s (restart the container): %w",
			ExecHelperPath, c.ID, err)
	}
	return nil
}
