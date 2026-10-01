# GoOOP IntelliJ plugin

Language support for GoOOP (`.goop`) files in IntelliJ IDEA and other JetBrains IDEs.

## What it gives you

- `.goop` files recognized as a first-class language (own file type + icon)
- Syntax highlighting: OOP keywords (`class`, `interface`, `abstract`, `extends`,
  `implements`, `constructor`, `override`, `new`, `this`, `super`, …), Go keywords,
  built-in types, strings, numbers, comments, operators
- Class-name highlighting after `class` / `interface` / `extends` / `implements` / `new`
- Brace / paren / bracket matching
- Line (`//`) and block (`/* */`) comment toggling
- A **Settings → Editor → Color Scheme → GoOOP** page to recolor every token

The lexer and highlighter are hand-written (no generated sources), and a flat
parser gives the file a PSI tree. Structure-aware features (go-to-definition,
folding, formatting) can be added later by replacing `GoopParser` with a real
grammar.

## Build & run

This is a standard [IntelliJ Platform Gradle Plugin](https://plugins.jetbrains.com/docs/intellij/tools-intellij-platform-gradle-plugin.html)
project. Requires a JDK 21 (Gradle downloads one automatically via the foojay
toolchain resolver).

Easiest path — **open `idea-plugin/` as a project in IntelliJ** (it will import
the Gradle build and download the wrapper), then use the Gradle tool window:

- **Run / debug the plugin:** `Tasks → intellij platform → runIde`
  (launches a sandbox IDE with the plugin installed — open a `.goop` file to test)
- **Build an installable zip:** `Tasks → intellij platform → buildPlugin`
  → output in `build/distributions/goop-intellij-0.1.0.zip`

From a terminal (once the wrapper exists):

```bash
cd idea-plugin
./gradlew runIde        # try it in a sandbox IDE
./gradlew buildPlugin   # produce the installable .zip
```

## Install the built plugin

In any JetBrains IDE: **Settings → Plugins → ⚙ → Install Plugin from Disk…**
and pick `build/distributions/goop-intellij-0.1.0.zip`.
