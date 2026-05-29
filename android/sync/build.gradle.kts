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

    // kotpass: KDBX-fil-håndtering. Bruges af Mapper.kt til at konvertere
    // mellem kotpass' typed Entry og vores canonical wire-format.
    implementation("app.keemobile:kotpass:0.13.0")

    // gomobile-bound Go-bro (compileOnly fordi JNI-libsene kun virker på
    // Android-runtime, ikke pure JVM — kun klasse-stubs trækkes ind til
    // compile-time så GomobileCryptoSession kan kompilere). Faktisk
    // runtime-binding sker når .aar'en inkluderes i :app-modulet.
    //
    // Genereres af `gomobile bind` (se ../README.md). Hvis filen mangler
    // fejler kompilation af GomobileCryptoSession; resten af :sync er
    // upåvirket.
    compileOnly(files("../libs/deltasync-classes.jar"))

    // OkHttp er den valgte HTTP-klient. Den deler okio (som kotpass også
    // bruger) — én transitiv dep, ingen extra-vægt.
    implementation("com.squareup.okhttp3:okhttp:4.12.0")

    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.3")
    testImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    // okio er en transitiv dep af kotpass, men dens types optræder kun
    // i nogle få kotpass-signaturer — eksplicit på testklassepathen så
    // vores fake binary store kan konstruere ByteString'er.
    testImplementation("com.squareup.okio:okio:3.15.0")
}

kotlin {
    // 21 matcher Android Studio's bundlede JBR. Bytecode-target er stadig
    // bagudkompatibel; vi behøver ikke Java 17 specifikt.
    jvmToolchain(21)
}

tasks.test {
    useJUnitPlatform()
}
