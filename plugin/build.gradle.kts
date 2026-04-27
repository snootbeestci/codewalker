plugins {
    id("org.jetbrains.kotlin.jvm") version "2.0.21"
    id("org.jetbrains.intellij") version "1.17.4"
}

group = "com.snootbeestci"
version = "0.1.0"

kotlin {
    jvmToolchain(17)
}

repositories {
    mavenCentral()
    maven {
        url = uri("https://maven.pkg.github.com/snootbeestci/codewalker")
        credentials {
            username = System.getenv("GITHUB_ACTOR") ?: ""
            password = System.getenv("GITHUB_TOKEN") ?: ""
        }
        content {
            includeGroup("com.github.snootbeestci")
        }
    }
}

intellij {
    version.set("2024.3")
    type.set("IC")
    plugins.set(emptyList())
    downloadSources.set(false)
}

dependencies {
    implementation("com.github.snootbeestci:codewalker-proto:0.1.9")
    implementation("io.grpc:grpc-netty-shaded:1.68.1")
    implementation("io.grpc:grpc-kotlin-stub:1.4.1")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.7.3")
}

tasks {
    patchPluginXml {
        sinceBuild.set("243")
        untilBuild.set("")
    }
}
