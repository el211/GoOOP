package goop

import com.intellij.extapi.psi.ASTWrapperPsiElement
import com.intellij.lang.ASTNode
import com.intellij.lang.ParserDefinition
import com.intellij.lang.PsiParser
import com.intellij.lexer.Lexer
import com.intellij.openapi.project.Project
import com.intellij.psi.FileViewProvider
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.TokenType
import com.intellij.psi.tree.IFileElementType
import com.intellij.psi.tree.TokenSet

class GoopParserDefinition : ParserDefinition {
    override fun createLexer(project: Project?): Lexer = GoopLexer()
    override fun createParser(project: Project?): PsiParser = GoopParser()
    override fun getFileNodeType(): IFileElementType = FILE
    override fun getCommentTokens(): TokenSet = COMMENTS
    override fun getStringLiteralElements(): TokenSet = STRINGS
    override fun getWhitespaceTokens(): TokenSet = WHITESPACE
    override fun createElement(node: ASTNode): PsiElement = ASTWrapperPsiElement(node)
    override fun createFile(viewProvider: FileViewProvider): PsiFile = GoopFile(viewProvider)

    companion object {
        val FILE = IFileElementType(GoopLanguage)
        val WHITESPACE: TokenSet = TokenSet.create(TokenType.WHITE_SPACE)
        val COMMENTS: TokenSet = TokenSet.create(GoopTypes.LINE_COMMENT, GoopTypes.BLOCK_COMMENT)
        val STRINGS: TokenSet = TokenSet.create(GoopTypes.STRING)
    }
}
