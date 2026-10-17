//go:build linux && amd64

package container

import "golang.org/x/sys/unix"

// AUDIT_ARCH_X86_64 — the arch field of struct seccomp_data on amd64.
// A mismatch means the syscall number table below is meaningless, so the
// filter denies instead of guessing.
const auditArchNative = unix.AUDIT_ARCH_X86_64

// blockX32Syscalls: the x32 ABI reports AUDIT_ARCH_X86_64 too but tags every
// syscall number with __X32_SYSCALL_BIT. Without rejecting that range the whole
// deny-list is bypassed by calling the same syscalls through x32.
const blockX32Syscalls = true

// syscallNumbers maps profile names to syscall numbers for this arch, taken
// from x/sys constants so the compiler rejects a typo or an arch mismatch
// instead of silently blocking the wrong number.
//
// Names missing from this arch's ABI are simply absent here; buildSeccompFilter
// skips them rather than failing, so one list can serve both arches.
var syscallNumbers = map[string]uint32{
	// Mount / namespace manipulation — the core escape primitives.
	// No umount/stime: both were removed from the x86_64 syscall table, so they
	// cannot be called here and need no filter entry.
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

	// Other-process memory and trace access — used to read or hijack
	// processes outside the container.
	"ptrace":            unix.SYS_PTRACE,
	"process_vm_readv":  unix.SYS_PROCESS_VM_READV,
	"process_vm_writev": unix.SYS_PROCESS_VM_WRITEV,
	"kcmp":              unix.SYS_KCMP,
	"pidfd_getfd":       unix.SYS_PIDFD_GETFD,

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
	"clock_adjtime": unix.SYS_CLOCK_ADJTIME,
	"settimeofday":  unix.SYS_SETTIMEOFDAY,
	"adjtimex":      unix.SYS_ADJTIMEX,
	"sethostname":   unix.SYS_SETHOSTNAME,
	"setdomainname": unix.SYS_SETDOMAINNAME,

	// Swap, log and raw port I/O.
	"swapoff": unix.SYS_SWAPOFF,
	"swapon":  unix.SYS_SWAPON,
	"syslog":  unix.SYS_SYSLOG,
	"ioperm":  unix.SYS_IOPERM,
	"iopl":    unix.SYS_IOPL,

	// open_by_handle_at / name_to_handle_at hand out filesystem handles that
	// bypass normal path checks — the other classic container escape.
	"open_by_handle_at": unix.SYS_OPEN_BY_HANDLE_AT,
	"name_to_handle_at": unix.SYS_NAME_TO_HANDLE_AT,
}
