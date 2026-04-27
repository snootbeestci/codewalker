plugins {
    kotlin("jvm") version "2.0.21"
    `maven-publish`
}

group = "com.github.snootbeestci"
version = System.getenv("RELEASE_VERSION") ?: "dev"

dependencies {
    implementation("io.grpc:grpc-kotlin-stub:1.4.1")
    implementation("io.grpc:grpc-protobuf-lite:1.60.0")
    implementation("com.google.protobuf:protobuf-kotlin-lite:3.25.1")
}

sourceSets {
    main {
        kotlin.srcDirs("../gen/kotlin")
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
