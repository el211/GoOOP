package goop

import com.intellij.lang.ASTNode
import com.intellij.lang.PsiBuilder
import com.intellij.lang.PsiParser
import com.intellij.psi.tree.IElementType

/**
 * A flat parser: it consumes every token into a single root node. This is enough
 * to give the file a PSI tree (so the platform treats .goop as a first-class
 * language), while the lexer/highlighter supply all the visible editor behaviour.
 * A real grammar (grammar-kit) can replace this later for structure-aware features.
 */
class GoopParser : PsiParser {
    override fun parse(root: IElementType, builder: PsiBuilder): ASTNode {
        val rootMarker = builder.mark()
        while (!builder.eof()) {
            builder.advanceLexer()
        }
        rootMarker.done(root)
        return builder.treeBuilt
    }
}
