//go:build linux

package container

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// SeccompAction represents a seccomp filter action.
type SeccompAction string

const (
	SeccompActionKill  SeccompAction = "SCMP_ACT_KILL"
	SeccompActionTrap  SeccompAction = "SCMP_ACT_TRAP"
	SeccompActionErrno SeccompAction = "SCMP_ACT_ERRNO"
	SeccompActionAllow SeccompAction = "SCMP_ACT_ALLOW"
	SeccompActionLog   SeccompAction = "SCMP_ACT_LOG"
)

// SeccompArch represents a seccomp architecture.
type SeccompArch string

const (
	SeccompArchX86_64 SeccompArch = "SCMP_ARCH_X86_64"
	SeccompArchX86    SeccompArch = "SCMP_ARCH_X86"
	SeccompArchARM64  SeccompArch = "SCMP_ARCH_AARCH64"
	SeccompArchALL    SeccompArch = "SCMP_ARCH_NATIVE"
)

// SeccompSyscall represents a single syscall rule.
type SeccompSyscall struct {
	Name   string        `json:"name"`
	Arch   []SeccompArch `json:"arch,omitempty"`
	Action SeccompAction `json:"action"`
}

// SeccompProfile represents a complete seccomp profile.
type SeccompProfile struct {
	DefaultAction SeccompAction    `json:"defaultAction"`
	Architectures []SeccompArch    `json:"architectures,omitempty"`
	Syscalls      []SeccompSyscall `json:"syscalls"`
}

// deniedSyscalls is the canonical deny-list. Names are resolved to numbers per
// arch by buildSeccompFilter; anything missing from an arch's ABI is skipped.
//
// Only syscalls that are both dangerous and never needed by a normal workload
// belong here. The previous version of this profile was deny-by-default with an
// allow-list that omitted futex, epoll_*, statx, newfstatat, rseq and others the
// Go runtime and glibc call unconditionally — compiling that faithfully would
// have killed every container on its first futex wait.
var deniedSyscalls = []string{
	// Mount / namespace manipulation — the core escape primitives.
	"mount", "umount2", "umount", "pivot_root", "chroot",

	// Kernel code loading and reboot.
	"init_module", "delete_module", "finit_module",
	"reboot", "kexec_load", "kexec_file_load",

	// Other-process memory and trace access — used to read or hijack
	// processes outside the container.
	"ptrace", "process_vm_readv", "process_vm_writev",
	"kcmp", "pidfd_getfd",

	// Kernel keyring.
	"keyctl", "add_key", "request_key",

	// Loading BPF and perf events — a kernel-exploitation primitive.
	"bpf", "perf_event_open", "userfaultfd",

	// Clock and hostname tampering.
	"clock_settime", "clock_adjtime", "settimeofday", "adjtimex", "stime",
	"sethostname", "setdomainname",

	// Swap, log and raw port I/O.
	"swapon", "swapoff", "syslog", "ioperm", "iopl",

	// open_by_handle_at / name_to_handle_at hand out filesystem handles that
	// bypass normal path checks — the other classic container escape.
	"open_by_handle_at", "name_to_handle_at",
}

// DefaultSeccompProfile returns the default seccomp profile.
//
// The model is allow-by-default: everything is permitted except the deny-list,
// which fails with EPERM. This mirrors Docker's default profile. The previous
// KILL-by-default shape is unsafe here because the allow-list could never be
// made complete against a changing Go runtime and glibc.
func DefaultSeccompProfile() *SeccompProfile {
	syscalls := make([]SeccompSyscall, 0, len(deniedSyscalls))
	for _, name := range deniedSyscalls {
		syscalls = append(syscalls, SeccompSyscall{
			Name:   name,
			Action: SeccompActionErrno,
		})
	}
	return &SeccompProfile{
		DefaultAction: SeccompActionAllow,
		Architectures: []SeccompArch{SeccompArchALL},
		Syscalls:      syscalls,
	}
}

// LoadSeccompProfile loads a seccomp profile from a JSON file.
func LoadSeccompProfile(path string) (*SeccompProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read seccomp profile: %w", err)
	}

	var profile SeccompProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse seccomp profile: %w", err)
	}

	return &profile, nil
}

// SaveSeccompProfile saves a seccomp profile to a JSON file.
func SaveSeccompProfile(profile *SeccompProfile, path string) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seccomp profile: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// ApplySeccompFilter applies a seccomp filter to the current process.
// This must be called after capabilities are set but before exec.
func ApplySeccompFilter(profile *SeccompProfile) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("seccomp only supported on Linux")
	}

	// Build the BPF filter from the profile
	filter, err := buildSeccompFilter(profile)
	if err != nil {
		return fmt.Errorf("build seccomp filter: %w", err)
	}

	// Set the filter using prctl
	// PR_SET_NO_NEW_PRIVS must be set before seccomp
	if err := setNoNewPrivileges(); err != nil {
		return fmt.Errorf("set no_new_privs: %w", err)
	}

	// PR_SET_SECCOMP
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_PRCTL,
		syscall.PR_SET_SECCOMP,
		unix.SECCOMP_MODE_FILTER,
		uintptr(unsafe.Pointer(&filter)),
		0, 0, 0,
	); errno != 0 {
		return fmt.Errorf("PR_SET_SECCOMP: %v", errno)
	}

	return nil
}

// seccompBPFInstruction represents a BPF instruction for seccomp.
type seccompBPFInstruction struct {
	Code uint16
	Jt   uint8
	Jf   uint8
	K    uint32
}

// seccompBPFProgram represents a BPF program for seccomp.
type seccompBPFProgram struct {
	Len    uint16
	Filter *seccompBPFInstruction
}

// Offsets into struct seccomp_data.
const (
	seccompDataNr   = 0
	seccompDataArch = 4
)

// Classic BPF opcodes used here (see linux/filter.h).
const (
	bpfLDWAbs = 0x20 // BPF_LD | BPF_W | BPF_ABS
	bpfJEQK   = 0x15 // BPF_JMP | BPF_JEQ | BPF_K
	bpfJGEK   = 0x35 // BPF_JMP | BPF_JGE | BPF_K
	bpfRetK   = 0x06 // BPF_RET | BPF_K
)

// seccomp return values (see linux/seccomp.h).
const (
	seccompRetErrnoMask = 0x00050000
	seccompRetAllow     = 0x7fff0000
	seccompRetErrnoEP   = 0x00050000 | uint32(unix.EPERM)
	seccompRetTrap      = 0x00030000
	seccompRetLog       = 0x7ffc0000
	seccompRetKillProc  = 0x80000000

	// __X32_SYSCALL_BIT — x32 syscalls share AUDIT_ARCH_X86_64, so without an
	// explicit reject of this range the deny-list can be bypassed via x32.
	x32SyscallBit = 0x40000000
)

// seccompRetAction maps a profile action to a seccomp return value. Default is
// ERRNO(EPERM) rather than KILL: a syscall we did not anticipate then fails one
// call instead of killing the container.
func seccompRetAction(action SeccompAction) uint32 {
	switch action {
	case SeccompActionAllow:
		return seccompRetAllow
	case SeccompActionKill:
		return seccompRetKillProc
	case SeccompActionTrap:
		return seccompRetTrap
	case SeccompActionLog:
		return seccompRetLog
	case SeccompActionErrno:
		return seccompRetErrnoEP
	default:
		return seccompRetErrnoEP
	}
}

func bpfStmt(code uint16, k uint32) seccompBPFInstruction {
	return seccompBPFInstruction{Code: code, K: k}
}

// bpfJump builds a conditional jump: if the test passes we skip jt
// instructions, otherwise jf.
func bpfJump(code uint16, k uint32, jt, jf uint8) seccompBPFInstruction {
	return seccompBPFInstruction{Code: code, K: k, Jt: jt, Jf: jf}
}

// buildSeccompProgram compiles a profile into cBPF instructions for struct
// seccomp_data on the native architecture.
//
// Layout:
//
//	LD  arch
//	JEQ <audit arch>, jt=1, jf=0
//	RET deny                       ← wrong arch: numbers mean nothing here
//	LD  nr
//	[JGE __X32_SYSCALL_BIT, jt=0, jf=1 ; RET deny]   ← x86_64 only
//	JEQ nr, <denied>, jt=0, jf=1 ; RET <action>      ← once per denied syscall
//	RET <default action>
func buildSeccompProgram(profile *SeccompProfile) ([]seccompBPFInstruction, error) {
	if profile == nil {
		return nil, fmt.Errorf("nil seccomp profile")
	}

	// Resolve names to numbers and drop duplicates. Names absent from this
	// arch's ABI (e.g. stime on amd64) simply do not resolve here.
	seen := make(map[uint32]bool)
	prog := []seccompBPFInstruction{
		bpfStmt(bpfLDWAbs, seccompDataArch),
		bpfJump(bpfJEQK, auditArchNative, 1, 0),
		bpfStmt(bpfRetK, seccompRetErrnoEP),
		bpfStmt(bpfLDWAbs, seccompDataNr),
	}
	if blockX32Syscalls {
		prog = append(prog,
			bpfJump(bpfJGEK, x32SyscallBit, 0, 1),
			bpfStmt(bpfRetK, seccompRetErrnoEP),
		)
	}

	rules := 0
	for _, sc := range profile.Syscalls {
		nr, ok := syscallNumbers[sc.Name]
		if !ok || seen[nr] {
			continue
		}
		seen[nr] = true
		prog = append(prog,
			bpfJump(bpfJEQK, nr, 0, 1),
			bpfStmt(bpfRetK, seccompRetAction(sc.Action)),
		)
		rules++
	}

	def := seccompRetAction(profile.DefaultAction)
	// A deny-by-default filter that resolved no rules would kill every
	// syscall, including the ones it should have allowed. Refuse it.
	if rules == 0 && def != seccompRetAllow {
		return nil, fmt.Errorf(
			"seccomp profile resolved 0 rules on this arch with default %s: refusing to install",
			profile.DefaultAction)
	}

	prog = append(prog, bpfStmt(bpfRetK, def))
	if len(prog) > 4098 {
		return nil, fmt.Errorf("seccomp program too long: %d instructions", len(prog))
	}
	return prog, nil
}

// buildSeccompFilter wraps the compiled program in the layout the kernel
// expects: struct sock_fprog { unsigned short len; struct sock_filter *filter; }.
// The pointer form is required — do not "simplify" it to a Go slice, that
// would change the struct layout and the kernel would read garbage.
func buildSeccompFilter(profile *SeccompProfile) (seccompBPFProgram, error) {
	prog, err := buildSeccompProgram(profile)
	if err != nil {
		return seccompBPFProgram{}, err
	}
	return seccompBPFProgram{Len: uint16(len(prog)), Filter: &prog[0]}, nil
}

// ApplyDefaultSeccompProfile applies the default seccomp profile to the
// current process. This is called during container initialization.
func ApplyDefaultSeccompProfile() error {
	profile := DefaultSeccompProfile()
	return ApplySeccompFilter(profile)
}

// SeccompSupported returns true if seccomp filtering is supported.
func SeccompSupported() bool {
	// Check if seccomp is available by reading /proc/self/status
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}

	// Look for Seccomp field
	for _, line := range splitLines(string(data)) {
		if len(line) > 9 && line[:9] == "Seccomp:" {
			// Seccomp mode: 0 = disabled, 1 = strict, 2 = filter
			// We can use mode 2 (filter)
			return true
		}
	}

	return true // Assume supported on modern kernels
}

// WriteDefaultSeccompProfile writes the default seccomp profile to a file.
func WriteDefaultSeccompProfile(path string) error {
	profile := DefaultSeccompProfile()
	return SaveSeccompProfile(profile, path)
}

// IsSeccompFilterActive checks if a seccomp filter is already active.
func IsSeccompFilterActive() bool {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}

	for _, line := range splitLines(string(data)) {
		if len(line) > 9 && line[:9] == "Seccomp:" {
			var mode int
			if _, err := fmt.Sscanf(line[9:], "%d", &mode); err == nil {
				return mode == 2 // SECCOMP_MODE_FILTER
			}
		}
	}

	return false
}
