Unicode true

####
## Thaw — modern one-click Windows installer (issue #825).
##
## This replaces the stock NSIS "wizard" that Wails generates when no
## build/windows/installer/project.nsi exists (the classic gray, multi-page,
## Program-Files, UAC-prompting wizard that looks nothing like Thaw).
##
## Instead this is an Electron/Squirrel-style one-click installer:
##   * per-user install to %LOCALAPPDATA%\Programs\Thaw — no UAC elevation,
##   * no Welcome / License / Directory / Finish pages — just a branded,
##     dark-themed progress page,
##   * bootstraps the WebView2 runtime if it is missing,
##   * launches Thaw automatically when the copy finishes.
##
## The output artifact name is unchanged (thaw-<arch>-installer.exe), so the
## CI pipeline (.github/workflows/build.yml) and Azure Trusted Signing steps
## need no changes.
##
## wails_tools.nsh is regenerated on every `wails build --nsis` from the
## ProjectInfo in wails.json — do NOT edit or commit it. This file, by
## contrast, is preserved by Wails once present and is the only place to
## customise the installer.
##
## Local development / debugging (from this directory, after one `wails build
## --target windows/amd64 --nsis` to populate wails_tools.nsh):
##   > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\thaw.exe project.nsi
####

####
## Per-user install: request "user" execution level so Windows never shows a
## UAC prompt, and install under the user's profile. The uninstall entry
## therefore lives under HKCU (writing HKLM would require elevation).
####
!define REQUEST_EXECUTION_LEVEL "user"

####
## Include the Wails-generated tools (defines INFO_*, PRODUCT_EXECUTABLE,
## UNINST_KEY, the wails.* macros, and sets RequestExecutionLevel).
####
!include "wails_tools.nsh"

# The version information for this must consist of 4 parts.
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI2.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"

####
## Branding: a dark, Thaw-coloured header logo, and a dark-themed progress
## list so the installer reads as part of the app rather than a stock wizard.
####
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "resources\header.bmp"
!define MUI_HEADERIMAGE_UNBITMAP "resources\header.bmp"
!define MUI_INSTFILESPAGE_COLORS "c9d1d9 0d1117"    # log text / background (Thaw dark theme)

# One-click flow: only the install-progress page (no Welcome/License/Directory/
# Finish). The page auto-closes on success and Thaw is launched below.
!insertmacro MUI_PAGE_INSTFILES

# The uninstaller keeps a single progress page too.
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

## The following two statements can be used to sign the installer and the
## uninstaller. In CI, signing is handled separately by Azure Trusted Signing.
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
BrandingText "${INFO_PRODUCTNAME} ${INFO_PRODUCTVERSION} — Snowflake Manager"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
# Modern per-user location (mirrors Electron apps): %LOCALAPPDATA%\Programs\Thaw
InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
ShowInstDetails show

Function .onInit
    !insertmacro wails.checkArchitecture
FunctionEnd

Section
    # Per-user shell context (Start Menu / Desktop under the current user).
    !insertmacro wails.setShellContext

    # Silently install the WebView2 runtime if it is not already present.
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    # Copy the architecture-appropriate binary as ${PRODUCT_EXECUTABLE}.
    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    # ── Per-user uninstall registration (Add/Remove Programs) ──────────────
    # A per-user install cannot write HKLM, so we register under HKCU rather
    # than using wails.writeUninstaller (which targets HKLM).
    WriteUninstaller "$INSTDIR\uninstall.exe"

    SetRegView 64
    WriteRegStr   HKCU "${UNINST_KEY}" "Publisher"        "${INFO_COMPANYNAME}"
    WriteRegStr   HKCU "${UNINST_KEY}" "DisplayName"      "${INFO_PRODUCTNAME}"
    WriteRegStr   HKCU "${UNINST_KEY}" "DisplayVersion"   "${INFO_PRODUCTVERSION}"
    WriteRegStr   HKCU "${UNINST_KEY}" "DisplayIcon"      "$INSTDIR\${PRODUCT_EXECUTABLE}"
    WriteRegStr   HKCU "${UNINST_KEY}" "InstallLocation"  "$INSTDIR"
    WriteRegStr   HKCU "${UNINST_KEY}" "UninstallString"      "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr   HKCU "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
    WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
    WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1

    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"

    # One-click finish: close the progress window and launch Thaw.
    SetAutoClose true
    Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    SetRegView 64
    DeleteRegKey HKCU "${UNINST_KEY}"
SectionEnd
