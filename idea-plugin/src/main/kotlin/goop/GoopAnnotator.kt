package goop

import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.Annotator
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.editor.DefaultLanguageHighlighterColors
import com.intellij.openapi.editor.colors.TextAttributesKey.createTextAttributesKey
import com.intellij.psi.PsiElement
import com.intellij.psi.util.PsiTreeUtil
import com.intellij.psi.util.elementType

/**
 * Lightweight semantic coloring on top of the lexer: the identifier that
 * follows a type-introducing keyword (class / interface / extends / implements /
 * new / constructor) is highlighted as a class name.
 */
class GoopAnnotator : Annotator {
    override fun annotate(element: PsiElement, holder: AnnotationHolder) {
        if (element.elementType != GoopTypes.IDENTIFIER) return

        val prev = PsiTreeUtil.prevLeaf(element, true) ?: return
        var cursor: PsiElement? = prev
        // Skip whitespace/comments between the keyword and the name.
        while (cursor != null &&
            (cursor.elementType == com.intellij.psi.TokenType.WHITE_SPACE ||
                cursor.elementType == GoopTypes.LINE_COMMENT ||
                cursor.elementType == GoopTypes.BLOCK_COMMENT)
        ) {
            cursor = PsiTreeUtil.prevLeaf(cursor, true)
        }
        if (cursor == null || cursor.elementType != GoopTypes.KEYWORD) return

        if (cursor.text in CLASS_INTRODUCERS) {
            holder.newSilentAnnotation(HighlightSeverity.INFORMATION)
                .range(element)
                .textAttributes(CLASS_NAME)
                .create()
        }
    }

    companion object {
        val CLASS_NAME = createTextAttributesKey(
            "GOOP_CLASS_NAME",
            DefaultLanguageHighlighterColors.CLASS_NAME,
        )
        private val CLASS_INTRODUCERS = setOf(
            "class", "interface", "extends", "implements", "new",
        )
    }
}
