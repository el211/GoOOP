package goop

import com.intellij.openapi.fileTypes.LanguageFileType
import com.intellij.openapi.util.IconLoader
import javax.swing.Icon

object GoopFileType : LanguageFileType(GoopLanguage) {
    // The Kotlin `object` exposes itself as the static field `INSTANCE`,
    // which plugin.xml references via fieldName="INSTANCE".

    val ICON: Icon = IconLoader.getIcon("/icons/goop.svg", GoopFileType::class.java)

    override fun getName(): String = "GoOOP file"
    override fun getDescription(): String = "GoOOP source file"
    override fun getDefaultExtension(): String = "goop"
    override fun getIcon(): Icon = ICON
}
