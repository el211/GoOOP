package goop

import com.intellij.lang.BracePair
import com.intellij.lang.PairedBraceMatcher
import com.intellij.psi.PsiFile
import com.intellij.psi.tree.IElementType

class GoopBraceMatcher : PairedBraceMatcher {
    override fun getPairs(): Array<BracePair> = PAIRS

    override fun isPairedBracesAllowedBeforeType(lbraceType: IElementType, contextType: IElementType?): Boolean = true

    override fun getCodeConstructStart(file: PsiFile?, openingBraceOffset: Int): Int = openingBraceOffset

    companion object {
        private val PAIRS = arrayOf(
            BracePair(GoopTypes.LBRACE, GoopTypes.RBRACE, true),
            BracePair(GoopTypes.LPAREN, GoopTypes.RPAREN, false),
            BracePair(GoopTypes.LBRACK, GoopTypes.RBRACK, false),
        )
    }
}
