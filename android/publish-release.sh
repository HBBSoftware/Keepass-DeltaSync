#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Publish a signed Android release APK to GitHub Releases (and optionally
# GitLab Releases) so Obtainium can install it and pick up updates.
#
# WHY THIS IS NOT A CI JOB: the release keystore (android/keystore.properties +
# deltasync-release.jks) is gitignored and deliberately never leaves this
# machine. CI would need it as a secret to produce an installable APK, so the
# build+sign step stays local and only the *upload* is scripted.
#
# The release TITLE must start with "DeltaSync Android" — that is what the
# published Obtainium config filters on to tell this app's releases apart from
# the client/* and server/* releases in the same monorepo. Don't rename it
# without updating android/README.md's Obtainium settings.
#
# Usage, from android/ after building the signed release APK:
#
#   ./gradlew :app:assembleRelease
#   GITHUB_TOKEN=ghp_xxx ./publish-release.sh
#
# Environment:
#   GITHUB_TOKEN    required — PAT with `repo` scope on the mirror
#   GITHUB_REPO     default HBBSoftware/Keepass-DeltaSync
#   GITLAB_TOKEN    optional — also publishes a GitLab Release (api scope)
#   GITLAB_PROJECT  default Star95%2Fkeepass-deltasync (URL-encoded path)
#   APK             optional — path to the APK, overrides autodetection
#   JAVA_HOME       needed by apksigner; autodetected from Android Studio
#   ANDROID_HOME    needed to find build-tools; autodetected
#   DRY_RUN=1       verify everything, upload nothing

set -eu

cd "$(dirname "$0")"

GITHUB_REPO="${GITHUB_REPO:-HBBSoftware/Keepass-DeltaSync}"
GITLAB_PROJECT="${GITLAB_PROJECT:-Star95%2Fkeepass-deltasync}"
DRY_RUN="${DRY_RUN:-0}"

die() { echo "error: $*" >&2; exit 1; }

# --- Locate the JDK and Android build-tools ---------------------------------
# apksigner is a shell wrapper around a JAR, so it needs a JDK. Android Studio
# ships one; fall back to whatever java is on PATH.
if [ -z "${JAVA_HOME:-}" ]; then
    for c in "/c/Program Files/Android/Android Studio/jbr" \
             "$HOME/AppData/Local/Programs/Android Studio/jbr"; do
        [ -d "$c" ] && { JAVA_HOME="$c"; export JAVA_HOME; break; }
    done
fi

SDK="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-${LOCALAPPDATA:-$HOME}/Android/Sdk}}"
APKSIGNER=""
AAPT2=""
# Glob order is lexical, so the last match is the newest build-tools release.
for c in "$SDK"/build-tools/*/apksigner "$SDK"/build-tools/*/apksigner.bat; do
    [ -f "$c" ] && APKSIGNER="$c"
done
for c in "$SDK"/build-tools/*/aapt2 "$SDK"/build-tools/*/aapt2.exe; do
    [ -f "$c" ] && AAPT2="$c"
done
[ -n "$APKSIGNER" ] || die "apksigner not found under $SDK/build-tools — set ANDROID_HOME"

# --- Read the version the build claims --------------------------------------
GRADLE=app/build.gradle.kts
VERSION_NAME=$(sed -n 's/^[[:space:]]*versionName[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$GRADLE")
VERSION_CODE=$(sed -n 's/^[[:space:]]*versionCode[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$GRADLE")
[ -n "$VERSION_NAME" ] || die "could not parse versionName from $GRADLE"
[ -n "$VERSION_CODE" ] || die "could not parse versionCode from $GRADLE"

TAG="android/v$VERSION_NAME"
TITLE="DeltaSync Android $VERSION_NAME"
ASSET="DeltaSync-$VERSION_NAME.apk"

echo "==> $TITLE (versionCode $VERSION_CODE), tag $TAG"

# --- Locate and sanity-check the APK ----------------------------------------
if [ -z "${APK:-}" ]; then
    for c in app/build/outputs/apk/release/*-release.apk; do
        [ -f "$c" ] && APK="$c"
    done
fi
[ -n "${APK:-}" ] && [ -f "$APK" ] || die "no release APK found — run ./gradlew :app:assembleRelease first"
echo "    APK: $APK ($(du -h "$APK" | cut -f1))"

# An unsigned APK cannot be installed, and a *differently* signed one silently
# breaks updates for everyone who already has the app. Both are worth failing on.
CERTS=$("$APKSIGNER" verify --print-certs "$APK" 2>/dev/null | grep 'SHA-256 digest' || true)
[ -n "$CERTS" ] || die "$APK is not signed — check android/keystore.properties"
CERT_SHA256=$(echo "$CERTS" | head -1 | sed 's/.*: *//')
echo "    signing cert SHA-256: $CERT_SHA256"

# The APK on disk can easily be older than the gradle file you just edited.
if [ -n "$AAPT2" ]; then
    BADGING=$("$AAPT2" dump badging "$APK" 2>/dev/null | head -1)
    APK_VN=$(echo "$BADGING" | sed -n "s/.*versionName='\([^']*\)'.*/\1/p")
    APK_VC=$(echo "$BADGING" | sed -n "s/.*versionCode='\([^']*\)'.*/\1/p")
    [ "$APK_VN" = "$VERSION_NAME" ] || die "APK versionName $APK_VN != gradle $VERSION_NAME — rebuild"
    [ "$APK_VC" = "$VERSION_CODE" ] || die "APK versionCode $APK_VC != gradle $VERSION_CODE — rebuild"
    echo "    APK version matches gradle: $APK_VN ($APK_VC)"
else
    echo "    WARNING: aapt2 not found — skipping APK-vs-gradle version check"
fi

# --- The tag must exist; Obtainium derives the version from it ---------------
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null \
    || die "tag $TAG does not exist — create it: git tag $TAG && git push origin $TAG"
if [ "$(git rev-parse "$TAG^{commit}")" != "$(git rev-parse HEAD)" ]; then
    echo "    WARNING: HEAD is not at $TAG — make sure the APK was built from the tag"
fi

# --- Stage the asset + checksum ---------------------------------------------
DIST=build/release-assets
rm -rf "$DIST" && mkdir -p "$DIST"
cp "$APK" "$DIST/$ASSET"
( cd "$DIST" && sha256sum "$ASSET" > SHA256SUMS )
echo "    staged $DIST/$ASSET"
cat "$DIST/SHA256SUMS" | sed 's/^/    /'

if [ "$DRY_RUN" = "1" ]; then
    echo "==> DRY_RUN=1, stopping before upload"
    exit 0
fi

# --- GitHub Release (this is the one Obtainium reads) ------------------------
[ -n "${GITHUB_TOKEN:-}" ] || die "GITHUB_TOKEN is not set"

API="https://api.github.com/repos/$GITHUB_REPO"
AUTH="Authorization: Bearer $GITHUB_TOKEN"
ACCEPT="Accept: application/vnd.github+json"

NOTES="Signed release APK for DeltaSync Android $VERSION_NAME (versionCode $VERSION_CODE).\n\nInstall or auto-update with Obtainium — see android/README.md.\nSigning certificate SHA-256: $CERT_SHA256\nVerify the download against SHA256SUMS."

body=$(printf '{"tag_name":"%s","name":"%s","body":"%s"}' "$TAG" "$TITLE" "$NOTES")
code=$(curl -sS -o /tmp/ds-rel.json -w '%{http_code}' -X POST "$API/releases" \
    -H "$AUTH" -H "$ACCEPT" -d "$body")
if [ "$code" = "201" ]; then
    echo "==> created GitHub release $TAG"
else
    echo "==> release create returned $code; reusing existing release for $TAG"
    curl -fsS "$API/releases/tags/$TAG" -H "$AUTH" -H "$ACCEPT" -o /tmp/ds-rel.json \
        || die "no release for $TAG and could not create one (see /tmp/ds-rel.json)"
fi

# First "id" in a GitHub release object is the release id (author.id comes later).
rid=$(grep -o '"id"[[:space:]]*:[[:space:]]*[0-9][0-9]*' /tmp/ds-rel.json | head -1 | tr -cd '0-9')
[ -n "$rid" ] || die "could not resolve release id — see /tmp/ds-rel.json"

existing=$(curl -fsS "$API/releases/$rid/assets" -H "$AUTH" -H "$ACCEPT" \
    | grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/')

for f in "$DIST/$ASSET" "$DIST/SHA256SUMS"; do
    name=$(basename "$f")
    case "$name" in *.apk) ctype="application/vnd.android.package-archive";; *) ctype="text/plain";; esac
    if echo "$existing" | grep -qx "$name"; then
        echo "    skip (already uploaded): $name"
        continue
    fi
    curl -fsS -X POST "https://uploads.github.com/repos/$GITHUB_REPO/releases/$rid/assets?name=$name" \
        -H "$AUTH" -H "Content-Type: $ctype" --data-binary @"$f" >/dev/null
    echo "    uploaded: $name"
done

echo "==> https://github.com/$GITHUB_REPO/releases/tag/$(echo "$TAG" | sed 's|/|%2F|')"

# --- GitLab Release (canonical repo; optional) -------------------------------
# The APK goes to the generic package registry first because that yields a
# stable direct-download URL we can hang off the release as an asset link.
if [ -n "${GITLAB_TOKEN:-}" ]; then
    GL="https://gitlab.com/api/v4/projects/$GITLAB_PROJECT"
    PKG="$GL/packages/generic/deltasync-android/$VERSION_NAME"
    for f in "$DIST/$ASSET" "$DIST/SHA256SUMS"; do
        name=$(basename "$f")
        curl -fsS --header "PRIVATE-TOKEN: $GITLAB_TOKEN" --upload-file "$f" "$PKG/$name" >/dev/null
        echo "    uploaded to GitLab package registry: $name"
    done

    rel=$(printf '{"name":"%s","tag_name":"%s","description":"%s","assets":{"links":[{"name":"%s","url":"%s/%s","link_type":"package"},{"name":"SHA256SUMS","url":"%s/SHA256SUMS","link_type":"other"}]}}' \
        "$TITLE" "$TAG" "$NOTES" "$ASSET" "$PKG" "$ASSET" "$PKG")
    code=$(curl -sS -o /tmp/ds-gl.json -w '%{http_code}' -X POST "$GL/releases" \
        -H "PRIVATE-TOKEN: $GITLAB_TOKEN" -H 'Content-Type: application/json' -d "$rel")
    case "$code" in
        20*) echo "==> created GitLab release $TAG" ;;
        409) echo "==> GitLab release $TAG already exists, left untouched" ;;
        *)   echo "    WARNING: GitLab release create returned $code (see /tmp/ds-gl.json)" ;;
    esac
else
    echo "==> GITLAB_TOKEN unset, skipping GitLab release"
fi

echo "==> done"
