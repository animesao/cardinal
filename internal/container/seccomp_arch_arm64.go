//go:build linux && arm64

package container

import "golang.org/x/sys/unix"

// AUDIT_ARCH_AARCH64 — the arch field of struct seccomp_data on arm64.
const auditArchNative = unix.AUDIT_ARCH_AARCH64

// arm64 has no x32-style secondary ABI: every syscall number is unique, so
// there is no tagged range to reject.
const blockX32Syscalls = false

// syscallNumbers maps profile names to syscall numbers for this arch, taken
// from x/sys constants so the compiler rejects a typo or an arch mismatch
// instead of silently blocking the wrong number.
//
// Syscalls absent from the arm64 ABI (stime, ioperm, iopl, umount, kcmp,
// pidfd_getfd, clock_adjtime) are deliberately missing: buildSeccompFilter skips
// names it cannot resolve, so the shared deny-list needs no per-arch branches.
var syscallNumbers = map[string]uint32{
	// Mount / namespace manipulation — the core escape primitives.
	"mount":      unix.SYS_MOUNT,
	"umount2":    unix.SYS_UMOUNT2,
	"pivot_root": unix.SYS_PIVOT_ROOT,
	"chroot":     unix.SYS_CHROOT,

	// Kernel code loading and reboot.
	"init_module":     unix.SYS_INIT_MODULE,
	"delete_module":   unix.SYS_DELETE_MODULE,
	"finit_module":    unix.SYS_FINIT_MODULE,
	"reboot":          unix.SYS_REBOOT,
	"kexec_load":      unix.SYS_KEXEC_LOAD,
	"kexec_file_load": unix.SYS_KEXEC_FILE_LOAD,

	// Other-process memory access.
	"ptrace":            unix.SYS_PTRACE,
	"process_vm_readv":  unix.SYS_PROCESS_VM_READV,
	"process_vm_writev": unix.SYS_PROCESS_VM_WRITEV,

	// Kernel keyring.
	"add_key":     unix.SYS_ADD_KEY,
	"request_key": unix.SYS_REQUEST_KEY,
	"keyctl":      unix.SYS_KEYCTL,

	// Loading BPF and perf events — a kernel-exploitation primitive.
	"bpf":             unix.SYS_BPF,
	"perf_event_open": unix.SYS_PERF_EVENT_OPEN,
	"userfaultfd":     unix.SYS_USERFAULTFD,

	// Clock and hostname tampering.
	"clock_settime": unix.SYS_CLOCK_SETTIME,
	"settimeofday":  unix.SYS_SETTIMEOFDAY,
	"adjtimex":      unix.SYS_ADJTIMEX,
	"sethostname":   unix.SYS_SETHOSTNAME,
	"setdomainname": unix.SYS_SETDOMAINNAME,

	// Swap and log.
	"swapoff": unix.SYS_SWAPOFF,
	"swapon":  unix.SYS_SWAPON,
	"syslog":  unix.SYS_SYSLOG,

	// open_by_handle_at / name_to_handle_at hand out filesystem handles that
	// bypass normal path checks — the other classic container escape.
	"open_by_handle_at": unix.SYS_OPEN_BY_HANDLE_AT,
	"name_to_handle_at": unix.SYS_NAME_TO_HANDLE_AT,
}
