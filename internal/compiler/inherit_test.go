package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runPkg(t *testing.T, mod string, files map[string]string) string {
	t.Helper()
	srcs := map[string][]byte{}
	for n, c := range files {
		srcs[n] = []byte(c)
	}
	outs, err := CompileFiles(srcs)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/"+mod+"\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for n, out := range outs {
		if err := os.WriteFile(filepath.Join(dir, n[:len(n)-5]+"_goop.go"), out, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	res, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, res)
	}
	return string(res)
}

func TestInheritedOverloadNotHidden(t *testing.T) {
	// Child declares Show(string); it must NOT hide the inherited Show(int).
	out := runPkg(t, "inhov", map[string]string{"m.goop": `package main
import "fmt"
public class Base {
    constructor() {}
    public Show(v int) string { return fmt.Sprintf("int:%d", v) }
}
public class Child extends Base {
    constructor() { super() }
    public Show(v string) string { return "str:" + v }
}
func main() {
    c := new Child()
    fmt.Println(c.Show(7))
    fmt.Println(c.Show("hi"))
}`})
	if out != "int:7\nstr:hi\n" {
		t.Fatalf("got %q", out)
	}
}

func TestMultiLevelInheritedOverload(t *testing.T) {
	out := runPkg(t, "multilvl", map[string]string{"m.goop": `package main
import "fmt"
public class A { constructor() {} public F(a int) string { return "a" } }
public class B extends A { constructor() { super() } public F(a string) string { return "b" } }
public class C extends B { constructor() { super() } public F(a bool) string { return "c" } }
func main() {
    x := new C()
    fmt.Println(x.F(1) + x.F("y") + x.F(true))
}`})
	if out != "abc\n" {
		t.Fatalf("got %q", out)
	}
}
