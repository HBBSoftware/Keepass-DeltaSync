// SPDX-License-Identifier: GPL-3.0-or-later

import java.io.FileInputStream
import java.util.Properties

plugins {
    id("com.android.application")
    kotlin("android")
    kotlin("plugin.serialization")
}

// Signing-config læses fra keystore.properties i android/ (gitignored). Findes
// den ikke, bygger release usigneret — så CI/F-Droid (der bruger egne nøgler)
// og rene debug-builds ikke kræver filen. Se README "Distribution".
val keystorePropsFile = rootProject.file("keystore.properties")
val keystoreProps = Properties().apply {
    if (keystorePropsFile.exists()) FileInputStream(keystorePropsFile).use { load(it) }
}

android {
    namespace = "dk.bjoerckbraun.deltasync"
    compileSdk = 35

    defaultConfig {
        applicationId = "dk.bjoerckbraun.deltasync"
        // 23 (Android 6.0) er nedre grænse: androidx.biometric kræver 23, og
        // Keystore-forankret EncryptedSharedPreferences er mere robust dér.
        minSdk = 23
        targetSdk = 35
        versionCode = 6
        versionName = "0.4.1"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        viewBinding = true
        buildConfig = true
    }

    signingConfigs {
        if (keystorePropsFile.exists()) {
            create("release") {
                // storeFile resolves relativt til android/ (rootProject).
                storeFile = rootProject.file(keystoreProps.getProperty("storeFile"))
                storePassword = keystoreProps.getProperty("storePassword")
                keyAlias = keystoreProps.getProperty("keyAlias")
                keyPassword = keystoreProps.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        getByName("release") {
            // Reproducible-build til F-Droid: ingen ProGuard/R8 i v0.1 så vi
            // har et frosset baseline at validere mod. Senere kan vi tilføje
            // shrinkResources + minifyEnabled når mappings er stabile.
            isMinifyEnabled = false
            // Signér release med vores keystore hvis konfigureret (ellers
            // usigneret — fx på CI/F-Droid der signerer selv).
            if (keystorePropsFile.exists()) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
        getByName("debug") {
            applicationIdSuffix = ".debug"
        }
    }

    packaging {
        resources {
            // gomobile-bound .aar genererer META-INF/-filer der kolliderer
            // ved samme navn. Tag den første og lad være med at fejle.
            excludes += setOf(
                "META-INF/AL2.0",
                "META-INF/LGPL2.1",
            )
        }
    }
}

dependencies {
    implementation(project(":sync"))

    // Den gomobile-bound .aar (genereret af `gomobile bind` — se ../README.md).
    // Skal være til stede før :app kan bygges; libs/ er gitignored.
    implementation(files("../libs/deltasync.aar"))

    // AndroidX core
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("com.google.android.material:material:1.12.0")
    implementation("androidx.activity:activity-ktx:1.9.2")

    // DataStore — for SyncStatePersistence impl.
    implementation("androidx.datastore:datastore-preferences:1.1.1")

    // EncryptedSharedPreferences — TokenStore impl.
    // Migreret fra androidx.security:security-crypto (deprecated) til den
    // modernere androidx.security:security-state hvis tilgængelig.
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // WorkManager — periodisk sync.
    implementation("androidx.work:work-runtime-ktx:2.9.1")

    // BiometricPrompt — gater opt-in til at huske kdbx-passwordet til
    // baggrunds-sync (se EncryptedPassphraseStore).
    implementation("androidx.biometric:biometric:1.1.0")

    // ZXing embedded — QR-scanning af enrollment-tokens (kamera). FOSS
    // (Apache-2.0), ingen Google Play Services → F-Droid-kompatibel, i
    // modsætning til ML Kit. Trækker com.google.zxing:core ind transitivt.
    implementation("com.journeyapps:zxing-android-embedded:4.3.0")

    // Coroutines — bruges af DataStore + WorkManager
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("org.jetbrains.kotlinx:kotlinx-datetime:0.6.1")
}
