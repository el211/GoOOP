package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageInheritanceGenericsInitializersAnnotations(t *testing.T) {
	sources := map[string][]byte{
		"animal.goop": []byte(`package main
@Reflect("root")
abstract class Animal[T any] {
    @Field("label")
    private label string = "animal"
    protected value T
    constructor(value T) { this.value = value }
    abstract Speak() string;
    Describe() string { return this.label + ":" + this.Speak() }
}`),
		"dog.goop": []byte(`package main
import "fmt"
@Serializable
class Dog extends Animal[string] {
    private count int = 3
    constructor(s string) { super(s) }
    @Override
    override Speak() string { return fmt.Sprintf("dog%d", this.count) }
}
func main() {
    d := new Dog("rex")
    fmt.Println(d.Describe())
    fmt.Println(GoOOPMetadataDog["annotations"])
}`),
	}
	outputs, err := CompileFiles(sources)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outputs["animal.goop"]), "type Animal[T any] struct") {
		t.Fatalf("generic type not generated:\n%s", outputs["animal.goop"])
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/advanced\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for path, out := range outputs {
		if err := os.WriteFile(filepath.Join(dir, strings.TrimSuffix(path, ".goop")+"_goop.go"), out, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if !strings.Contains(string(result), "animal:dog3") || !strings.Contains(string(result), "Serializable") {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestOverrideAllowsParameterRename(t *testing.T) {
	src := `package main
class Parent { virtual Sum(first int, second int) int { return first+second } }
class Child extends Parent { override Sum(a int,b int) int { return a+b } }
func main(){ _ = new Child().Sum(1,2) }`
	if _, err := Compile("rename.goop", []byte(src)); err != nil {
		t.Fatal(err)
	}
}

func TestGenericOverrideSpecialization(t *testing.T) {
	src := `package main
abstract class Base[T any] { abstract Value() T; }
class Child extends Base[string] { override Value() string {return "ok"} }
func main(){_ = new Child().Value()}`
	if _, err := Compile("special.goop", []byte(src)); err != nil {
		t.Fatal(err)
	}
}

func TestSubclassCannotAccessPrivateMembers(t *testing.T) {
	src := `package main
class Parent { private secret string = "hidden" }
class Child extends Parent { Read() string {return this.secret} }`
	_, err := Compile("privacy.goop", []byte(src))
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("expected private access rejection: %v", err)
	}
}
