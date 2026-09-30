package goop

import com.intellij.lang.Language

object GoopLanguage : Language("GoOOP") {
    private fun readResolve(): Any = GoopLanguage
    override fun getDisplayName(): String = "GoOOP"
}
