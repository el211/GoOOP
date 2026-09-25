// Command goop is the GoOOP compiler and project CLI.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/el211/GoOOP/internal/compiler"
)

const version = "0.3.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "goop:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "init":
		if len(args) < 2 || len(args) > 3 {
			return errors.New("usage: goop init <module-path> [directory]")
		}
		dir := "."
		if len(args) == 3 {
			dir = args[2]
		}
		return initProject(args[1], dir)
	case "generate", "gen":
		paths := args[1:]
		if len(paths) == 0 {
			paths = []string{"."}
		}
		n, err := generate(paths)
		if err != nil {
			return err
		}
		fmt.Printf("GoOOP: processed %d file(s)\n", n)
		return nil
	case "check", "build", "run", "test":
		overlay, n, cleanup, err := temporaryOverlay()
		if err != nil {
			return err
		}
		defer cleanup()
		_ = n
		flag := "-overlay=" + overlay
		switch args[0] {
		case "check":
			return goCommand("build", flag, "./...")
		case "build":
			if err := goCommand("build", flag, "./..."); err != nil {
				return err
			}
			// Like go build ., emit a binary when the current package is main.
			// Library modules are validated without creating a meaningless file.
			name := exec.Command("go", "list", flag, "-f", "{{.Name}}", ".")
			out, err := name.Output()
			if err != nil {
				return fmt.Errorf("inspect current Go package: %w", err)
			}
			if strings.TrimSpace(string(out)) != "main" {
				return nil
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			binary := filepath.Base(cwd)
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			return goCommand("build", flag, "-o", filepath.Join(cwd, binary), ".")
		case "run":
			return goCommand(append([]string{"run", flag, "."}, args[1:]...)...)
		case "test":
			// Go's overlay supports virtual source files, but its go test vet
			// step tries to open their disk paths directly. Disable implicit vet.
			// Use goop generate + go vet ./... for an explicit vet pass.
			return goCommand(append([]string{"test", "-vet=off", flag, "./..."}, args[1:]...)...)
		}
		return nil
	case "clean":
		if len(args) != 1 {
			return errors.New("usage: goop clean")
		}
		n, err := cleanExports()
		if err != nil {
			return err
		}
		fmt.Printf("GoOOP: removed %d exported generated file(s); temporary build files are deleted automatically\n", n)
		return nil
	case "version", "--version", "-v":
		fmt.Println("GoOOP", version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try goop help)", args[0])
	}
}
func goCommand(args ...string) error {
	c := exec.Command("go", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func usage() {
	fmt.Print(`GoOOP — classical OOP syntax, native Go output

Usage:
  goop init <module-path> [directory]  Create a new project
  goop build                         Build via temporary Go sources; output binary for main packages
  goop run [args...]                 Run using temporary Go sources
  goop test [go test flags...]       Test using temporary Go sources
  goop check                         Type-check/build using temporary Go sources
  goop generate [files/directories]  Explicitly export *_goop.go files to source directories
  goop clean                         Remove explicitly exported GoOOP-generated files
  goop version                       Print compiler version

Build, run, test and check never write generated Go source into your project.
GoOOP uses Go's -overlay flag and automatically removes its temp files.
`)
}
func initProject(module, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	gomod := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(gomod); err == nil {
		return fmt.Errorf("%s already exists", gomod)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sample := filepath.Join(dir, "app.goop")
	if _, err := os.Stat(sample); err == nil {
		return fmt.Errorf("%s already exists", sample)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.WriteFile(gomod, []byte("module "+module+"\n\ngo 1.22\n"), 0644); err != nil {
		return err
	}
	const example = `package main

import "fmt"

class Animal {
    private name string

    constructor(name string) {
        this.name = name
    }

    virtual Speak() string {
        return "Some sound"
    }

    GetName() string {
        return this.name
    }

    Describe() string {
        return this.GetName() + ": " + this.Speak()
    }
}

class Dog extends Animal {
    constructor(name string) {
        super(name)
    }

    override Speak() string {
        return "Woof!"
    }
}

func main() {
    dog := new Dog("Rex")
    fmt.Println(dog.Describe())
}
`
	if err := os.WriteFile(sample, []byte(example), 0644); err != nil {
		return err
	}
	fmt.Printf("Initialized %s in %s\nNext: goop run\n", module, dir)
	return nil
}

// generate explicitly exports selected .goop sources. Sibling .goop files
// are parsed for package-level resolution, but not exported unless requested.
func generate(paths []string) (int, error) {
	selected := map[string]bool{}
	for _, entry := range paths {
		info, err := os.Stat(entry)
		if err != nil {
			return 0, err
		}
		if !info.IsDir() {
			if !strings.HasSuffix(entry, ".goop") {
				return 0, fmt.Errorf("expected .goop file: %s", entry)
			}
			absolute, err := filepath.Abs(entry)
			if err != nil {
				return 0, err
			}
			selected[absolute] = true
			continue
		}
		err = filepath.WalkDir(entry, func(name string, item fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if item.IsDir() {
				if name != entry && (item.Name() == ".git" || item.Name() == "vendor" || item.Name() == "node_modules" || strings.HasPrefix(item.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(item.Name(), ".goop") {
				absolute, err := filepath.Abs(name)
				if err != nil {
					return err
				}
				selected[absolute] = true
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	sources := map[string][]byte{}
	for path := range selected {
		siblings, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.goop"))
		if err != nil {
			return 0, err
		}
		for _, sibling := range siblings {
			absolute, err := filepath.Abs(sibling)
			if err != nil {
				return 0, err
			}
			data, err := os.ReadFile(absolute)
			if err != nil {
				return 0, err
			}
			sources[absolute] = data
		}
	}
	if len(selected) == 0 {
		return 0, nil
	}
	outputs, err := compiler.CompileFiles(sources)
	if err != nil {
		return 0, err
	}
	ordered := make([]string, 0, len(selected))
	for path := range selected {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	count := 0
	for _, path := range ordered {
		target := strings.TrimSuffix(path, ".goop") + "_goop.go"
		output := outputs[path]
		old, err := os.ReadFile(target)
		if err == nil && bytes.Equal(old, output) {
			count++
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return count, err
		}
		if err := os.WriteFile(target, output, 0644); err != nil {
			return count, err
		}
		fmt.Printf("generated %s\n", target)
		count++
	}
	return count, nil
}
