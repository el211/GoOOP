package goop

import com.intellij.lexer.Lexer
import com.intellij.openapi.editor.DefaultLanguageHighlighterColors
import com.intellij.openapi.editor.colors.TextAttributesKey
import com.intellij.openapi.editor.colors.TextAttributesKey.createTextAttributesKey
import com.intellij.openapi.fileTypes.SyntaxHighlighterBase
import com.intellij.psi.TokenType
import com.intellij.psi.tree.IElementType

class GoopSyntaxHighlighter : SyntaxHighlighterBase() {
    override fun getHighlightingLexer(): Lexer = GoopLexer()

    override fun getTokenHighlights(tokenType: IElementType): Array<TextAttributesKey> = when (tokenType) {
        GoopTypes.KEYWORD -> KEYWORD_KEYS
        GoopTypes.TYPE -> TYPE_KEYS
        GoopTypes.CONSTANT -> CONSTANT_KEYS
        GoopTypes.ANNOTATION -> ANNOTATION_KEYS
        GoopTypes.NUMBER -> NUMBER_KEYS
        GoopTypes.STRING -> STRING_KEYS
        GoopTypes.LINE_COMMENT -> LINE_COMMENT_KEYS
        GoopTypes.BLOCK_COMMENT -> BLOCK_COMMENT_KEYS
        GoopTypes.LPAREN, GoopTypes.RPAREN -> PAREN_KEYS
        GoopTypes.LBRACE, GoopTypes.RBRACE -> BRACE_KEYS
        GoopTypes.LBRACK, GoopTypes.RBRACK -> BRACKET_KEYS
        GoopTypes.SEMICOLON -> SEMICOLON_KEYS
        GoopTypes.OPERATOR -> OPERATOR_KEYS
        GoopTypes.IDENTIFIER -> IDENTIFIER_KEYS
        TokenType.BAD_CHARACTER -> BAD_CHAR_KEYS
        else -> EMPTY_KEYS
    }

    companion object {
        val KEYWORD = key("GOOP_KEYWORD", DefaultLanguageHighlighterColors.KEYWORD)
        val TYPE = key("GOOP_TYPE", DefaultLanguageHighlighterColors.NUMBER)
        val CONSTANT = key("GOOP_CONSTANT", DefaultLanguageHighlighterColors.CONSTANT)
        val ANNOTATION = key("GOOP_ANNOTATION", DefaultLanguageHighlighterColors.METADATA)
        val IDENTIFIER = key("GOOP_IDENTIFIER", DefaultLanguageHighlighterColors.IDENTIFIER)
        val NUMBER = key("GOOP_NUMBER", DefaultLanguageHighlighterColors.NUMBER)
        val STRING = key("GOOP_STRING", DefaultLanguageHighlighterColors.STRING)
        val LINE_COMMENT = key("GOOP_LINE_COMMENT", DefaultLanguageHighlighterColors.LINE_COMMENT)
        val BLOCK_COMMENT = key("GOOP_BLOCK_COMMENT", DefaultLanguageHighlighterColors.BLOCK_COMMENT)
        val PARENTHESES = key("GOOP_PARENTHESES", DefaultLanguageHighlighterColors.PARENTHESES)
        val BRACES = key("GOOP_BRACES", DefaultLanguageHighlighterColors.BRACES)
        val BRACKETS = key("GOOP_BRACKETS", DefaultLanguageHighlighterColors.BRACKETS)
        val SEMICOLON = key("GOOP_SEMICOLON", DefaultLanguageHighlighterColors.SEMICOLON)
        val OPERATOR = key("GOOP_OPERATOR", DefaultLanguageHighlighterColors.OPERATION_SIGN)
        val BAD_CHARACTER = key("GOOP_BAD_CHARACTER", com.intellij.openapi.editor.HighlighterColors.BAD_CHARACTER)

        private fun key(name: String, fallback: TextAttributesKey) = createTextAttributesKey(name, fallback)

        private val KEYWORD_KEYS = arrayOf(KEYWORD)
        private val TYPE_KEYS = arrayOf(TYPE)
        private val CONSTANT_KEYS = arrayOf(CONSTANT)
        private val ANNOTATION_KEYS = arrayOf(ANNOTATION)
        private val IDENTIFIER_KEYS = arrayOf(IDENTIFIER)
        private val NUMBER_KEYS = arrayOf(NUMBER)
        private val STRING_KEYS = arrayOf(STRING)
        private val LINE_COMMENT_KEYS = arrayOf(LINE_COMMENT)
        private val BLOCK_COMMENT_KEYS = arrayOf(BLOCK_COMMENT)
        private val PAREN_KEYS = arrayOf(PARENTHESES)
        private val BRACE_KEYS = arrayOf(BRACES)
        private val BRACKET_KEYS = arrayOf(BRACKETS)
        private val SEMICOLON_KEYS = arrayOf(SEMICOLON)
        private val OPERATOR_KEYS = arrayOf(OPERATOR)
        private val BAD_CHAR_KEYS = arrayOf(BAD_CHARACTER)
        private val EMPTY_KEYS = emptyArray<TextAttributesKey>()
    }
}
