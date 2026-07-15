$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class WindowCaptureNative {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int command);
    [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdc, uint flags);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extraInfo);
}
"@

function Save-WindowScreenshot([IntPtr]$handle, [string]$path) {
    $rect = New-Object WindowCaptureNative+RECT
    if (-not [WindowCaptureNative]::GetWindowRect($handle, [ref]$rect)) {
        throw "Unable to read application window bounds"
    }
    $width = $rect.Right - $rect.Left
    $height = $rect.Bottom - $rect.Top
    $bitmap = New-Object System.Drawing.Bitmap $width, $height
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
        $hdc = $graphics.GetHdc()
        try {
            $captured = [WindowCaptureNative]::PrintWindow($handle, $hdc, 2)
        } finally {
            $graphics.ReleaseHdc($hdc)
        }
        if (-not $captured) {
            (New-Object -ComObject WScript.Shell).AppActivate($handle.ToInt32()) | Out-Null
            Start-Sleep -Milliseconds 500
            $graphics.CopyFromScreen($rect.Left, $rect.Top, 0, 0, $bitmap.Size)
        }
        $bitmap.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    } finally {
        $graphics.Dispose()
        $bitmap.Dispose()
    }
}

function Save-ScreenshotCrop([string]$source, [string]$path) {
    $image = [System.Drawing.Image]::FromFile($source)
    try {
        $cropWidth = [Math]::Min(1100, $image.Width)
        $cropHeight = [Math]::Min(760, $image.Height - 100)
        $bitmap = New-Object System.Drawing.Bitmap $cropWidth, $cropHeight
        $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
        try {
            $sourceRect = New-Object System.Drawing.Rectangle 0, 100, $cropWidth, $cropHeight
            $destinationRect = New-Object System.Drawing.Rectangle 0, 0, $cropWidth, $cropHeight
            $graphics.DrawImage($image, $destinationRect, $sourceRect, [System.Drawing.GraphicsUnit]::Pixel)
            $bitmap.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
        } finally {
            $graphics.Dispose()
            $bitmap.Dispose()
        }
    } finally {
        $image.Dispose()
    }
}

$root = Split-Path -Parent $PSScriptRoot
$output = Join-Path $root "docs\images"
New-Item -ItemType Directory -Force $output | Out-Null
$process = Start-Process -FilePath (Join-Path $root "bin\ResumeStudio.exe") -PassThru

try {
    for ($attempt = 0; $attempt -lt 30 -and $process.MainWindowHandle -eq 0; $attempt++) {
        Start-Sleep -Milliseconds 500
        $process.Refresh()
    }
    if ($process.MainWindowHandle -eq 0) { throw "Resume Studio window did not appear" }

    [WindowCaptureNative]::SetForegroundWindow($process.MainWindowHandle) | Out-Null
    Start-Sleep -Seconds 6
    [WindowCaptureNative]::ShowWindow($process.MainWindowHandle, 3) | Out-Null
    Start-Sleep -Seconds 4
    $focusScreenshot = Join-Path $output "resume-studio-focus.png"
    Save-WindowScreenshot $process.MainWindowHandle $focusScreenshot
    Save-ScreenshotCrop $focusScreenshot (Join-Path $output "resume-studio-editor.png")
} finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
    Get-Process tectonic -ErrorAction SilentlyContinue | Stop-Process -Force
}
