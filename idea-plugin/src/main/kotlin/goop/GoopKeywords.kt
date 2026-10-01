package goop

object GoopKeywords {
    // GoOOP object-oriented extensions + Go control/declaration keywords.
    val KEYWORDS: Set<String> = setOf(
        // OOP extensions
        "class", "interface", "enum", "abstract", "extends", "implements",
        "constructor", "override", "new", "this", "super",
        "private", "public", "protected", "static", "final", "virtual",
        // Go keywords
        "break", "case", "chan", "const", "continue", "default", "defer",
        "else", "fallthrough", "for", "func", "go", "goto", "if", "import",
        "map", "package", "range", "return", "select", "struct", "switch",
        "type", "var",
    )

    // Built-in type names, highlighted distinctly.
    val TYPES: Set<String> = setOf(
        "bool", "string", "error", "any",
        "int", "int8", "int16", "int32", "int64",
        "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
        "byte", "rune",
        "float32", "float64", "complex64", "complex128",
    )

    // Predeclared constants.
    val CONSTANTS: Set<String> = setOf(
        "true", "false", "nil", "iota",
    )
}
