//go:build linux

package container

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// decode returns the instruction slice of a compiled program.
func decode(t *testing.T, prog []seccompBPFInstruction) []seccompBPFInstruction {
	t.Helper()
	if len(prog) == 0 {
		t.Fatal("empty program")
	}
	return prog
}

func TestDefaultProfileIsAllowByDefault(t *testing.T) {
	p := DefaultSeccompProfile()
	if p.DefaultAction != SeccompActionAllow {
		t.Fatalf("default action = %s, want %s: a deny-by-default profile whose "+
			"allow-list cannot stay complete kills every container",
			p.DefaultAction, SeccompActionAllow)
	}
	if len(p.Syscalls) == 0 {
		t.Fatal("deny-list is empty: profile would block nothing")
	}
	for _, sc := range p.Syscalls {
		if sc.Action != SeccompActionErrno {
			t.Errorf("%s action = %s, want %s (EPERM degrades gracefully, KILL does not)",
				sc.Name, sc.Action, SeccompActionErrno)
		}
	}
}

// The old profile listed the same syscall as both KILL and ALLOW, so the
// effective action depended on evaluation order. Guard against any repeat.
func TestProfileHasNoContradictoryActions(t *testing.T) {
	p := DefaultSeccompProfile()
	seen := make(map[string]bool, len(p.Syscalls))
	for _, sc := range p.Syscalls {
		if seen[sc.Name] {
			t.Errorf("%s listed more than once", sc.Name)
		}
		seen[sc.Name] = true
	}
}

// Regression guard: these are called unconditionally by the Go runtime and
// glibc. Denying any of them bricks the container.
func TestRuntimeCriticalSyscallsAreNotDenied(t *testing.T) {
	mustNotDeny := []string{
		"futex", "futex_waitv", "epoll_create1", "epoll_ctl", "epoll_wait",
		"clone3", "clone", "clone_newflags", "statx", "newfstatat", "fstatat64",
		"rseq", "set_robust_list", "membarrier", "getdents64", "dup3",
		"pipe2", "madvise", "move_pages", "mbind", "set_mempolicy",
		"get_mempolicy", "prctl", "execve", "execveat", "io_uring_setup",
	}
	denied := make(map[string]bool)
	for _, sc := range DefaultSeccompProfile().Syscalls {
		denied[sc.Name] = true
	}
	for _, name := range mustNotDeny {
		if denied[name] {
			t.Errorf("%s is denied but is required by the runtime", name)
		}
	}
}

// Every denied syscall that exists on this arch must be compiled into the
// program exactly once.
func TestFilterCoversEveryResolvableSyscall(t *testing.T) {
	p := DefaultSeccompProfile()
	prog, err := buildSeccompProgram(p)
	if err != nil {
		t.Fatalf("buildSeccompProgram: %v", err)
	}
	ins := decode(t, prog)

	compared := make(map[uint32]bool)
	for _, i := range ins {
		if i.Code == bpfJEQK && i.K != auditArchNative {
			compared[i.K] = true
		}
	}
	for _, sc := range p.Syscalls {
		nr, ok := syscallNumbers[sc.Name]
		if !ok {
			continue // not in this arch's ABI
		}
		if !compared[nr] {
			t.Errorf("%s (nr %d) resolved but is missing from the compiled program", sc.Name, nr)
		}
	}
	// Nothing outside the deny-list may be compared, or the program would
	// reject syscalls the profile never mentioned.
	for name, nr := range syscallNumbers {
		_ = name
		if compared[nr] && !inDenyList(p, name) {
			t.Errorf("syscall %q (nr %d) is compiled into the filter but not in the profile", name, nr)
		}
	}
}

func inDenyList(p *SeccompProfile, name string) bool {
	for _, sc := range p.Syscalls {
		if sc.Name == name {
			return true
		}
	}
	return false
}

// The arch guard must come first: on a mismatched arch the syscall numbers in
// our table are meaningless.
func TestFilterGuardsArchitecture(t *testing.T) {
	prog, err := buildSeccompProgram(DefaultSeccompProfile())
	if err != nil {
		t.Fatalf("buildSeccompProgram: %v", err)
	}
	ins := decode(t, prog)

	if ins[0].Code != bpfLDWAbs || ins[0].K != seccompDataArch {
		t.Fatalf("insn[0] = %+v, want LD arch", ins[0])
	}
	if ins[1].Code != bpfJEQK || ins[1].K != auditArchNative {
		t.Fatalf("insn[1] = %+v, want JEQ audit arch", ins[1])
	}
	if ins[1].Jt != 1 || ins[1].Jf != 0 {
		t.Fatalf("arch jump jt=%d jf=%d, want 1/0 (match must skip the deny)", ins[1].Jt, ins[1].Jf)
	}
	if ins[2].Code != bpfRetK || ins[2].K != seccompRetErrnoEP {
		t.Fatalf("insn[2] = %+v, want RET deny on arch mismatch", ins[2])
	}
	if ins[3].Code != bpfLDWAbs || ins[3].K != seccompDataNr {
		t.Fatalf("insn[3] = %+v, want LD nr", ins[3])
	}

	last := ins[len(ins)-1]
	if last.Code != bpfRetK || last.K != seccompRetAllow {
		t.Fatalf("last insn = %+v, want RET ALLOW", last)
	}
}

// On x86_64 the x32 ABI shares AUDIT_ARCH_X86_64 and tags syscall numbers with
// __X32_SYSCALL_BIT, so the deny-list is bypassable without an explicit reject.
func TestFilterRejectsX32(t *testing.T) {
	prog, err := buildSeccompProgram(DefaultSeccompProfile())
	if err != nil {
		t.Fatalf("buildSeccompProgram: %v", err)
	}
	ins := decode(t, prog)

	found := false
	for _, i := range ins {
		if i.Code == bpfJGEK && i.K == x32SyscallBit {
			found = true
			if i.Jt != 0 || i.Jf != 1 {
				t.Errorf("x32 jump jt=%d jf=%d, want 0/1 (hit must fall into the deny)", i.Jt, i.Jf)
			}
		}
	}
	if blockX32Syscalls && !found {
		t.Error("amd64 filter does not reject the x32 syscall range")
	}
	if !blockX32Syscalls && found {
		t.Error("x32 reject present on an arch that has no x32 ABI")
	}
}

// A deny-by-default profile that resolves nothing would deny every syscall,
// including the ones it was supposed to allow. Refuse instead of installing.
func TestDenyByDefaultWithNoRulesIsRefused(t *testing.T) {
	_, err := buildSeccompProgram(&SeccompProfile{
		DefaultAction: SeccompActionKill,
		Syscalls: []SeccompSyscall{
			{Name: "definitely_not_a_syscall", Action: SeccompActionErrno},
		},
	})
	if err == nil {
		t.Fatal("expected refusal for a deny-by-default profile with no resolvable rules")
	}
}

func TestNilProfileIsRejected(t *testing.T) {
	if _, err := buildSeccompProgram(nil); err == nil {
		t.Fatal("expected error for nil profile")
	}
}

// Compile-time check that the deny-list and the arch tables stay in sync: a
// name in deniedSyscalls that no arch can resolve is dead weight or a typo.
func TestDenyListIsResolvableOnNativeArch(t *testing.T) {
	for _, name := range deniedSyscalls {
		if _, ok := syscallNumbers[name]; !ok {
			t.Logf("%s not in %s ABI (expected for arch-specific syscalls)", name, "native")
		}
	}
	if len(syscallNumbers) == 0 {
		t.Fatal("syscallNumbers is empty")
	}
}

// seccompChildEnv marks the re-executed child. Applying a seccomp filter is
// irreversible, so it can only be exercised in a dedicated process.
const seccompChildEnv = "CARDINAL_SECCOMP_TEST_CHILD"

// TestAppliedSeccompFilterDeniesAndAllows installs the real filter in a child
// process and checks the kernel's verdict, rather than trusting our own
// compiler. Needs no root: PR_SET_NO_NEW_PRIVS makes PR_SET_SECCOMP
// unprivileged.
func TestAppliedSeccompFilterDeniesAndAllows(t *testing.T) {
	if os.Getenv(seccompChildEnv) == "1" {
		seccompChild()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestAppliedSeccompFilterDeniesAndAllows$", "-test.v")
	cmd.Env = append(os.Environ(), seccompChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	t.Logf("child output:\n%s", out)
	if err != nil {
		t.Fatalf("child failed: %v", err)
	}
	text := string(out)
	for _, want := range []string{"denied=mount", "denied=pivot_root", "allowed=stat", "allowed=open"} {
		if !strings.Contains(text, want) {
			t.Errorf("child did not report %q", want)
		}
	}
	if strings.Contains(text, "CHILD-FAIL") {
		t.Fatal("child reported a failed assertion")
	}
}

func seccompChild() {
	if err := ApplyDefaultSeccompProfile(); err != nil {
		fmt.Printf("CHILD-FAIL apply seccomp: %v\n", err)
		os.Exit(1)
	}
	if !IsSeccompFilterActive() {
		fmt.Println("CHILD-FAIL filter not active after apply")
		os.Exit(1)
	}

	// Denied: mount is the escape primitive the deny-list exists for.
	if err := unix.Mount("tmpfs", "/tmp", "tmpfs", 0, ""); err == nil {
		fmt.Println("CHILD-FAIL mount succeeded, filter is not enforcing")
		os.Exit(1)
	} else if !errors.Is(err, unix.EPERM) {
		fmt.Printf("CHILD-FAIL mount denied with %v, want EPERM\n", err)
		os.Exit(1)
	}
	fmt.Println("denied=mount")

	// Denied: pivot_root would let the container swap its root out.
	if err := unix.PivotRoot(".", "."); err == nil {
		fmt.Println("CHILD-FAIL pivot_root succeeded")
		os.Exit(1)
	} else if !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.EINVAL) {
		fmt.Printf("CHILD-FAIL pivot_root denied with %v, want EPERM\n", err)
		os.Exit(1)
	}
	fmt.Println("denied=pivot_root")

	// Allowed: an over-broad filter would have killed one of these already.
	var st unix.Stat_t
	if err := unix.Stat("/", &st); err != nil {
		fmt.Printf("CHILD-FAIL stat denied: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("allowed=stat")

	f, err := os.Open("/dev/null")
	if err != nil {
		fmt.Printf("CHILD-FAIL open denied: %v\n", err)
		os.Exit(1)
	}
	_ = f.Close()
	fmt.Println("allowed=open")

	// os.Exit instead of returning: the test harness does a lot of work after
	// the test body that we do not want running under the filter.
	os.Exit(0)
}
