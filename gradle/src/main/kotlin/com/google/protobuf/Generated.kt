package com.google.protobuf

// Shim: the buf.build remote plugins may emit @com.google.protobuf.Generated on generated
// files/classes before the annotation class is published to Maven Central. This stub
// satisfies the compiler reference for both Kotlin (@file:) and Java (class-level) usages.
@Target(
    AnnotationTarget.FILE,   // Kotlin @file:Generated
    AnnotationTarget.CLASS,  // Java class-level @Generated
)
@Retention(AnnotationRetention.BINARY)
annotation class Generated
