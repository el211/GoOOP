package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultipleInheritanceExplicitParents(t *testing.T) {
	src := `package main
import "fmt"
class Named {
  private name string
  constructor(name string) { this.name=name }
  Name() string {return this.name}
}
class Numbered {
  private num int = 7
  constructor(num int) {this.num=num}
  Number() int{return this.num}
}
class Combined extends Named, Numbered {
  constructor(name string,num int) {
     super(name);
     super.Numbered(num)
  }
  Describe() string {return fmt.Sprintf("%s:%d",this.Name(),this.Number())}
}
func main(){ fmt.Println(new Combined("Rex",3).Describe()) }`
	out, err := Compile("multiple.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/multi\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "app_goop.go"), out, 0644)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, out)
	}
	if string(result) != "Rex:3\n" {
		t.Fatalf("result: %s", result)
	}
}
func TestMultipleInheritanceRequiresDisambiguation(t *testing.T) {
	src := `package main
class A { Foo() string {return "a"} }
class B { Foo() string {return "b"} }
class C extends A, B {}`
	_, err := Compile("collision.goop", []byte(src))
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity diagnostic; got %v", err)
	}
}
