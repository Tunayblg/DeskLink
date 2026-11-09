plugins {
    id("com.android.application")
}

// Release builds are signed with the key given in these environment variables
// (CI sets them from repository secrets). Every release must use the same key,
// or Android refuses to update an installed copy. Without them, the build falls
// back to the debug key, which is fine for trying the app out.
val releaseKeystore: String? = System.getenv("DESKLINK_KEYSTORE")

android {
    namespace = "com.desklink.app"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.desklink.app"
        minSdk = 21
        targetSdk = 34
        versionCode = 3
        versionName = "1.2.0"
    }

    signingConfigs {
        if (releaseKeystore != null) {
            create("release") {
                storeFile = file(releaseKeystore)
                storeType = "pkcs12"
                storePassword = System.getenv("DESKLINK_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("DESKLINK_KEY_ALIAS")
                keyPassword = System.getenv("DESKLINK_KEYSTORE_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    // Google's QR scanner. It runs inside Google Play services, so the app needs no camera permission.
    implementation("com.google.android.gms:play-services-code-scanner:16.1.0")
}
