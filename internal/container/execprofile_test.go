//go:build linux

package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultProfileHasNoDangerousCaps — главное утверждение безопасности
// exec-пути: capability-набор контейнера по умолчанию не содержит
// CAP_SYS_ADMIN и CAP_MKNOD.
//
// Именно эта пара давала выход из контейнера: mknod создавал ноду блочного
// устройства, а CAP_SYS_ADMIN (в начальном user namespace, т.к. контейнеры
// работают без user namespace) позволял его смонтировать.
func TestDefaultProfileHasNoDangerousCaps(t *testing.T) {
	c := &Container{ID: "test", Name: "test"}
	p, err := buildExecProfile(c)
	if err != nil {
		t.Fatalf("buildExecProfile: %v", err)
	}
	for _, bad := range []string{"SYS_ADMIN", "MKNOD", "SYS_MODULE", "SYS_RAWIO", "SYS_PTRACE"} {
		for _, have := range p.Capabilities {
			if have == bad {
				t.Errorf("профиль содержит опасную capability %s: %v", bad, p.Capabilities)
			}
		}
	}
	// Базовый набор должен остаться рабочим (иначе сломалось бы всё).
	found := false
	for _, have := range p.Capabilities {
		if have == "CHOWN" {
			found = true
		}
	}
	if !found {
		t.Errorf("в профиле нет CHOWN: %v", p.Capabilities)
	}
}

func TestProfileCapAddAndDrop(t *testing.T) {
	c := &Container{ID: "test", CapAdd: []string{"NET_ADMIN"}}
	p, err := buildExecProfile(c)
	if err != nil {
		t.Fatalf("buildExecProfile: %v", err)
	}
	has := func(name string) bool {
		for _, h := range p.Capabilities {
			if h == name {
				return true
			}
		}
		return false
	}
	if !has("NET_ADMIN") {
		t.Errorf("CapAdd не применился: %v", p.Capabilities)
	}
	if has("SYS_ADMIN") {
		t.Errorf("SYS_ADMIN не должен появляться из CapAdd: %v", p.Capabilities)
	}

	c2 := &Container{ID: "test", CapDrop: []string{"CHOWN"}}
	p2, err := buildExecProfile(c2)
	if err != nil {
		t.Fatalf("buildExecProfile: %v", err)
	}
	for _, h := range p2.Capabilities {
		if h == "CHOWN" {
			t.Errorf("CapDrop не применился: %v", p2.Capabilities)
		}
	}
}

func TestProfileRejectsUnknownCapability(t *testing.T) {
	c := &Container{ID: "test", CapAdd: []string{"NOT_A_CAP"}}
	if _, err := buildExecProfile(c); err == nil {
		t.Fatal("ожидалась ошибка на неизвестной capability")
	}
}

func TestProfileUser(t *testing.T) {
	c := &Container{ID: "test", User: "1000:1000"}
	p, err := buildExecProfile(c)
	if err != nil {
		t.Fatalf("buildExecProfile: %v", err)
	}
	if !p.HasUser || p.UID != 1000 || p.GID != 1000 {
		t.Errorf("пользователь контейнера не перенесён: %+v", p)
	}
	// setuid/setgid нужны самому процессу, чтобы сменить пользователя.
	has := false
	for _, h := range p.Capabilities {
		if h == "SETUID" {
			has = true
		}
	}
	if !has {
		t.Errorf("в профиле нет SETUID: %v", p.Capabilities)
	}

	c2 := &Container{ID: "test", User: "1000"}
	p2, err := buildExecProfile(c2)
	if err != nil {
		t.Fatalf("buildExecProfile: %v", err)
	}
	if !p2.HasUser || p2.UID != 1000 {
		t.Errorf("uid без gid: %+v", p2)
	}
}

// TestExecEnvHasNoDaemonSecrets — C2: окружение демона не должно попадать
// внутрь контейнера. В нём живут CARDINAL_TOKEN (root-эквивалентный токен
// API демона), CARDINAL_BACKUP_KEY и DOCKER_PASSWORD.
func TestExecEnvHasNoDaemonSecrets(t *testing.T) {
	t.Setenv("CARDINAL_TOKEN", "super-secret-token")
	t.Setenv("DOCKER_PASSWORD", "super-secret-password")
	t.Setenv("CARDINAL_BACKUP_KEY", "super-secret-key")

	p := &ExecProfile{Env: []string{"PATH=/usr/bin", "GAMEMODE=survival"}}
	env := execEnv(p)

	joined := strings.Join(env, "\n")
	for _, secret := range []string{"CARDINAL_TOKEN", "DOCKER_PASSWORD", "CARDINAL_BACKUP_KEY", "super-secret"} {
		if strings.Contains(joined, secret) {
			t.Errorf("секрет демона попал в окружение контейнера: %q", secret)
		}
	}
	if !strings.Contains(joined, "GAMEMODE=survival") {
		t.Errorf("окружение контейнера потерялось: %v", env)
	}
}

func TestExecEnvAlwaysHasPathHomeTerm(t *testing.T) {
	p := &ExecProfile{Env: []string{"GAMEMODE=survival"}}
	env := strings.Join(execEnv(p), "\n")
	for _, want := range []string{"PATH=", "HOME=", "TERM="} {
		if !strings.Contains(env, want) {
			t.Errorf("в окружении нет %s: %v", want, env)
		}
	}
}

// TestExecEnvKeepsContainerPath — если у контейнера свой PATH, он не
// подменяется дефолтным.
func TestExecEnvKeepsContainerPath(t *testing.T) {
	p := &ExecProfile{Env: []string{"PATH=/opt/java/bin"}}
	env := strings.Join(execEnv(p), "\n")
	if !strings.Contains(env, "PATH=/opt/java/bin") {
		t.Errorf("PATH контейнера подменён: %v", env)
	}
	if strings.Count(env, "PATH=") != 1 {
		t.Errorf("PATH продублирован: %v", env)
	}
}

func TestStageAndLoadExecProfile(t *testing.T) {
	dir := t.TempDir()
	c := &Container{
		ID:         "test",
		User:       "1000:1000",
		CapAdd:     []string{"NET_ADMIN"},
		Env:        []string{"PATH=/usr/bin"},
		WorkingDir: "/home/container",
	}
	if err := stageExecProfile(c, dir); err != nil {
		t.Fatalf("stageExecProfile: %v", err)
	}
	// stageExecProfile кладёт профиль в <merged>/cardinal/ — там же, где
	// живут остальные утилиты cardinal внутри контейнера.
	path := filepath.Join(dir, strings.TrimPrefix(ExecHelperDir, "/"), filepath.Base(ExecProfilePath))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("профиль не записан: %v", err)
	}
	p, err := loadExecProfile(path)
	if err != nil {
		t.Fatalf("loadExecProfile: %v", err)
	}
	if !p.HasUser || p.UID != 1000 {
		t.Errorf("пользователь не сохранился: %+v", p)
	}
	if p.WorkingDir != "/home/container" {
		t.Errorf("working dir не сохранился: %+v", p)
	}
	if len(p.Capabilities) == 0 {
		t.Errorf("capabilities пусты: %+v", p)
	}
	// Профиль читается только владельцем.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("профиль доступен группе/остальным: %v", info.Mode().Perm())
	}
}

func TestExecHelperRejectsBadInvocations(t *testing.T) {
	if code := ExecHelperMain(nil); code != 2 {
		t.Errorf("код выхода без команды = %d, ожидался 2", code)
	}
	if code := ExecHelperMain([]string{"--profile"}); code != 2 {
		t.Errorf("код выхода с --profile без значения = %d, ожидался 2", code)
	}
	// Несуществующий профиль => отказ, а не exec без ограничений.
	if code := ExecHelperMain([]string{"--profile", "/nonexistent/profile.json", "--", "/bin/true"}); code != 1 {
		t.Errorf("код выхода с битым профилем = %d, ожидался 1", code)
	}
}

// TestResolveCommandFindsShortNames — короткие имена ("sh") должны
// находиться в PATH: syscall.Exec, в отличие от nsenter, PATH не ищет.
func TestResolveCommandFindsShortNames(t *testing.T) {
	env := []string{"PATH=/usr/bin:/bin"}
	got, err := resolveCommand("sh", env)
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if !strings.HasPrefix(got, "/") {
		t.Errorf("команда не разрешена в абсолютный путь: %q", got)
	}
	// Путь с разделителем разрешается как есть.
	if got, err := resolveCommand("/bin/sh", env); err != nil || got != "/bin/sh" {
		t.Errorf("абсолютный путь изменён: %q %v", got, err)
	}
	// Несуществующей команды быть не должно.
	if _, err := resolveCommand("definitely-not-a-real-binary-xyz", env); err == nil {
		t.Error("ожидалась ошибка для несуществующей команды")
	}
}
