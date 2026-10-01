package goop

import com.intellij.extapi.psi.PsiFileBase
import com.intellij.openapi.fileTypes.FileType
import com.intellij.psi.FileViewProvider

class GoopFile(viewProvider: FileViewProvider) : PsiFileBase(viewProvider, GoopLanguage) {
    override fun getFileType(): FileType = GoopFileType
    override fun toString(): String = "GoOOP File"
}
