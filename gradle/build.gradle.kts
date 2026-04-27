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
    implementation("com.google.protobuf:protobuf-java:3.25.1")
    implementation("com.google.protobuf:protobuf-kotlin:3.25.1")
    implementation("io.grpc:grpc-stub:1.60.0")
    implementation("io.grpc:grpc-protobuf:1.60.0")
    implementation("io.grpc:grpc-kotlin-stub:1.4.1")
    compileOnly("javax.annotation:javax.annotation-api:1.3.2")
}

sourceSets {
    main {
        kotlin.srcDirs("../gen/kotlin")
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
