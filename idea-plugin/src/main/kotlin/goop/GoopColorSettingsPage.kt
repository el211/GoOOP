package goop

import com.intellij.openapi.editor.colors.TextAttributesKey
import com.intellij.openapi.fileTypes.SyntaxHighlighter
import com.intellij.openapi.options.colors.AttributesDescriptor
import com.intellij.openapi.options.colors.ColorDescriptor
import com.intellij.openapi.options.colors.ColorSettingsPage
import javax.swing.Icon

class GoopColorSettingsPage : ColorSettingsPage {
    override fun getIcon(): Icon = GoopFileType.ICON
    override fun getHighlighter(): SyntaxHighlighter = GoopSyntaxHighlighter()
    override fun getAdditionalHighlightingTagToDescriptorMap(): Map<String, TextAttributesKey> = TAGS
    override fun getAttributeDescriptors(): Array<AttributesDescriptor> = DESCRIPTORS
    override fun getColorDescriptors(): Array<ColorDescriptor> = ColorDescriptor.EMPTY_ARRAY
    override fun getDisplayName(): String = "GoOOP"

    override fun getDemoText(): String = """
        package main

        import "fmt"

        // A speaking thing
        interface Speaker {
            Speak() string;
        }

        abstract class <class>Animal</class> {
            private name string

            constructor(name string) {
                this.name = name
            }

            abstract Speak() string;

            GetName() string {
                return this.name
            }
        }

        class <class>Dog</class> extends <class>Animal</class> implements <class>Speaker</class> {
            override Speak() string {
                return "woof"
            }
        }

        func main() {
            dog := new <class>Dog</class>("Rex")
            fmt.Println(dog.GetName())
            count := 42
            _ = count
        }
    """.trimIndent()

    companion object {
        private val DESCRIPTORS = arrayOf(
            AttributesDescriptor("Keyword", GoopSyntaxHighlighter.KEYWORD),
            AttributesDescriptor("Built-in type", GoopSyntaxHighlighter.TYPE),
            AttributesDescriptor("Constant", GoopSyntaxHighlighter.CONSTANT),
            AttributesDescriptor("Annotation", GoopSyntaxHighlighter.ANNOTATION),
            AttributesDescriptor("Identifier", GoopSyntaxHighlighter.IDENTIFIER),
            AttributesDescriptor("Class name", GoopAnnotator.CLASS_NAME),
            AttributesDescriptor("Number", GoopSyntaxHighlighter.NUMBER),
            AttributesDescriptor("String", GoopSyntaxHighlighter.STRING),
            AttributesDescriptor("Line comment", GoopSyntaxHighlighter.LINE_COMMENT),
            AttributesDescriptor("Block comment", GoopSyntaxHighlighter.BLOCK_COMMENT),
            AttributesDescriptor("Parentheses", GoopSyntaxHighlighter.PARENTHESES),
            AttributesDescriptor("Braces", GoopSyntaxHighlighter.BRACES),
            AttributesDescriptor("Brackets", GoopSyntaxHighlighter.BRACKETS),
            AttributesDescriptor("Semicolon", GoopSyntaxHighlighter.SEMICOLON),
            AttributesDescriptor("Operator", GoopSyntaxHighlighter.OPERATOR),
            AttributesDescriptor("Bad character", GoopSyntaxHighlighter.BAD_CHARACTER),
        )

        private val TAGS = mapOf("class" to GoopAnnotator.CLASS_NAME)
    }
}
