// :sync er pure-JVM Kotlin — ingen Android-afhængigheder. Indeholder canonical
// wire-format-typer + (senere) kotpass-mapper. Tests kører på almindelig JVM,
// så vi behøver ikke emulator for at validere mod Go-emitterede fixtures.
//
// Når Android-app-modulet (:app) tilføjes, vil det depende på dette modul.

plugins {
    kotlin("jvm")
    kotlin("plugin.serialization")
}

dependencies {
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("org.jetbrains.kotlinx:kotlinx-datetime:0.6.1")

    // kotpass tilføjes når Kotlin-side mapperen skrives (Phase C3).
    // implementation("com.github.keemobile:kotpass:0.10.0")

    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.3")
}

kotlin {
    // 21 matcher Android Studio's bundlede JBR. Bytecode-target er stadig
    // bagudkompatibel; vi behøver ikke Java 17 specifikt.
    jvmToolchain(21)
}

tasks.test {
    useJUnitPlatform()
}
