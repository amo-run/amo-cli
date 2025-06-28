#!/usr/bin/env python3
"""
Generate download page from HTML template.
This script reads the template and replaces placeholders with actual content.
Also copies installation scripts to the site directory.
"""

import os
import sys
import glob
import subprocess
import shutil
from pathlib import Path

def get_file_size(filepath):
    """Get human-readable file size."""
    try:
        size = os.path.getsize(filepath)
        for unit in ['B', 'KB', 'MB', 'GB']:
            if size < 1024.0:
                return f"{size:.1f}{unit}"
            size /= 1024.0
        return f"{size:.1f}TB"
    except:
        return "Unknown"

def get_arch_info(filename):
    """Get architecture information from filename."""
    if 'amd64' in filename:
        if 'linux' in filename:
            return '64-bit'
        elif 'darwin' in filename:
            return 'Intel'
        elif 'windows' in filename:
            return '64-bit'
    elif 'arm64' in filename:
        if 'linux' in filename:
            return 'ARM64'
        elif 'darwin' in filename:
            return 'Apple Silicon'
    elif 'armv7' in filename:
        return 'ARMv7'
    return 'Unknown'

def generate_platform_section(platform_name, platform_icon, file_pattern):
    """Generate HTML section for a platform."""
    files = glob.glob(file_pattern)
    files = [f for f in files if not f.endswith('.sha256') and os.path.isfile(f)]
    
    if not files:
        return ""
    
    section = f'''                <div class="platform-card">
                    <h3><span class="platform-icon">{platform_icon}</span>{platform_name}</h3>'''
    
    for filepath in files:
        filename = os.path.basename(filepath)
        filesize = get_file_size(filepath)
        arch_info = get_arch_info(filename)
        
        section += f'''
                    <a href="{filename}" class="download-link">{filename}</a>
                    <div class="file-info">{arch_info} • {filesize}</div>'''
    
    section += '''
                </div>'''
    
    return section

def generate_checksums_section():
    """Generate checksums section."""
    sha_files = glob.glob("*.sha256")
    if not sha_files:
        return ""
    
    checksums = ""
    for sha_file in sha_files:
        binary_name = sha_file.replace('.sha256', '')
        if os.path.isfile(binary_name):
            try:
                with open(sha_file, 'r') as f:
                    checksum = f.read().strip().split()[0]
                checksums += f'''
                <div class="checksum-item">
                    <div class="checksum-filename">{binary_name}</div>
                    <div class="checksum-hash">{checksum}</div>
                </div>'''
            except:
                continue
    
    return checksums

def copy_installation_scripts():
    """Copy installation scripts to the site directory."""
    script_dir = Path(__file__).parent
    
    # Installation scripts to copy
    scripts = [
        ("install.sh", "Unix/Linux/macOS installation script"),
        ("install.ps1", "Windows PowerShell installation script"),
        ("install.bat", "Windows Batch installation script")
    ]
    
    for script_name, description in scripts:
        src_path = script_dir / script_name
        if src_path.exists():
            # Copy to current directory (site)
            shutil.copy2(src_path, script_name)
            print(f"✅ Copied {script_name} ({description})")
        else:
            print(f"⚠️ Warning: {script_name} not found at {src_path}")

def main():
    if len(sys.argv) != 4:
        print("Usage: generate-download-page.py <template_file> <tag_name> <release_name>")
        sys.exit(1)
    
    template_file = sys.argv[1]
    tag_name = sys.argv[2]
    release_name = sys.argv[3]
    
    # Read template
    try:
        with open(template_file, 'r') as f:
            content = f.read()
    except FileNotFoundError:
        print(f"Error: Template file {template_file} not found")
        sys.exit(1)
    
    # Generate platform sections
    linux_section = generate_platform_section("Linux", "🐧", "amo_linux_*")
    macos_section = generate_platform_section("macOS", "🍎", "amo_darwin_*")
    windows_section = generate_platform_section("Windows", "🪟", "amo_windows_*.exe")
    
    download_sections = linux_section + macos_section + windows_section
    
    # Generate checksums
    checksums = generate_checksums_section()
    
    # Replace placeholders
    content = content.replace('{{TAG_NAME}}', tag_name)
    content = content.replace('{{RELEASE_NAME}}', release_name)
    content = content.replace('{{DOWNLOAD_SECTIONS}}', download_sections)
    content = content.replace('{{CHECKSUMS}}', checksums)
    
    # Write output
    with open('index.html', 'w') as f:
        f.write(content)
    
    print("✅ Generated index.html successfully")
    
    # Copy installation scripts
    copy_installation_scripts()

if __name__ == "__main__":
    main() 