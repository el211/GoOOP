package goop

import com.intellij.psi.tree.IElementType

class GoopTokenType(debugName: String) : IElementType(debugName, GoopLanguage) {
    override fun toString(): String = "GoopTokenType." + super.toString()
}

class GoopElementType(debugName: String) : IElementType(debugName, GoopLanguage)

object GoopTypes {
    // Words
    val KEYWORD = GoopTokenType("KEYWORD")
    val TYPE = GoopTokenType("TYPE")
    val CONSTANT = GoopTokenType("CONSTANT")
    val ANNOTATION = GoopTokenType("ANNOTATION")
    val IDENTIFIER = GoopTokenType("IDENTIFIER")

    // Literals
    val STRING = GoopTokenType("STRING")
    val NUMBER = GoopTokenType("NUMBER")

    // Comments
    val LINE_COMMENT = GoopTokenType("LINE_COMMENT")
    val BLOCK_COMMENT = GoopTokenType("BLOCK_COMMENT")

    // Punctuation
    val LPAREN = GoopTokenType("LPAREN")
    val RPAREN = GoopTokenType("RPAREN")
    val LBRACE = GoopTokenType("LBRACE")
    val RBRACE = GoopTokenType("RBRACE")
    val LBRACK = GoopTokenType("LBRACK")
    val RBRACK = GoopTokenType("RBRACK")
    val SEMICOLON = GoopTokenType("SEMICOLON")
    val OPERATOR = GoopTokenType("OPERATOR")
}
