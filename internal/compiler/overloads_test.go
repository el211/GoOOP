package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConstructorOverloadsByArity(t *testing.T) {
	src := `package main
import "fmt"
class Dog {
    private name string = "Milo"
    private age int = 1
    constructor() { }
    constructor(name string) { this.name = name }
    constructor(name string, age int) { this.name=name; this.age=age }
    Describe() string {return fmt.Sprintf("%s:%d",this.name,this.age)}
}
func main(){
    fmt.Println(new Dog().Describe())
    fmt.Println(new Dog("Rex").Describe())
    fmt.Println(new Dog("Rex",4).Describe())
}`
	generated, err := Compile("overloads.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/overloads\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "overloads_goop.go"), generated, 0644)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", src, err, out)
	}
	if string(out) != "Milo:1\nRex:1\nRex:4\n" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestMethodOverloadsDistinctArity(t *testing.T) {
	src := `package main
import "fmt"
class Calculator {
    public Add(a int) int {return a+1}
    public Add(a int,b int) int {return a+b}
    constructor() {}
    Summary() int {return this.Add(2)+this.Add(3,4)}
}
func main(){
    calc:=new Calculator()
    fmt.Println(calc.Summary())
    fmt.Println(calc.Add(9,1))
}`
	generated, err := Compile("methods.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/methods\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "methods_goop.go"), generated, 0644)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s\n%s", err, out, generated)
	}
	if string(out) != "10\n10\n" {
		t.Fatalf("got %q", out)
	}
}
