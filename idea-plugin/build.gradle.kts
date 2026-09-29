plugins {
    kotlin("jvm") version "2.0.21"
    id("org.jetbrains.intellij.platform") version "2.1.0"
}

group = "com.goop"
version = "0.1.0"

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        // Build against IntelliJ IDEA Community. Any 2024.3+ IDE (including the
        // JetBrains products you already run) can install the resulting plugin.
        intellijIdeaCommunity("2024.3")
    }
}

kotlin {
    jvmToolchain(21)
}

intellijPlatform {
    // No .form files / NotNull instrumentation in this plugin — skip it.
    instrumentCode = false
    // Skip headless searchable-options build; not needed for a language plugin.
    buildSearchableOptions = false

    pluginConfiguration {
        ideaVersion {
            sinceBuild = "243"
            // Leave untilBuild open so the plugin keeps working on newer IDEs.
            untilBuild = provider { null }
        }
    }
}
