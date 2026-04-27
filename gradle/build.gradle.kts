plugins {
    kotlin("jvm") version "2.0.21"
    `maven-publish`
}

repositories {
    mavenCentral()
    google()
}

group = "com.github.snootbeestci"
version = System.getenv("RELEASE_VERSION") ?: "dev"

dependencies {
    implementation("com.google.protobuf:protobuf-java:3.25.3")
    implementation("com.google.protobuf:protobuf-kotlin:3.25.3")
    implementation("io.grpc:grpc-stub:1.68.1")
    implementation("io.grpc:grpc-protobuf:1.68.1")
    implementation("io.grpc:grpc-kotlin-stub:1.4.1")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.7.3")
    implementation("javax.annotation:javax.annotation-api:1.3.2")
}

sourceSets {
    main {
        kotlin.srcDirs("../gen/kotlin", "src/main/kotlin")
        java.srcDirs("../gen/java")
    }
}

publishing {
    publications {
        create<MavenPublication>("stubs") {
            artifactId = "codewalker-proto"
            from(components["kotlin"])
        }
    }
    repositories {
        maven {
            name = "GitHubPackages"
            url = uri("https://maven.pkg.github.com/snootbeestci/codewalker")
            credentials {
                username = System.getenv("GITHUB_ACTOR")
                password = System.getenv("GITHUB_TOKEN")
            }
        }
    }
}
