//!amo
/**
 * ImageMagick Windows Automated Installer Workflow
 * 
 * This workflow automatically downloads and installs ImageMagick portable version
 * with region-based mirror selection for optimal download speeds.
 * 
 * Features:
 * 1. Scrapes ImageMagick official website for latest portable version
 * 2. Uses mirror for China region users
 * 3. Falls back to official site if mirror fails
 * 4. Automatically extracts ZIP and sets up PATH
 * 5. Validates installation and cleans up temporary files
 */

const WORKFLOW_NAME = "imagemagick-windows-installer";
const IMAGEMAGICK_BASE_URL = "https://imagemagick.org/archive/binaries/";
const MIRROR_BASE_URL = "https://toolchains.mirror.toulan.fun/software/";

// Region detection based on system environment
function detectRegion() {
    try {
        // Get system locale from environment
        const locale = getVar('LANG') || getVar('LC_ALL') || 'en_US.UTF-8';
        
        // Check locale
        if (locale && (locale.startsWith('zh') || locale.startsWith('cn'))) {
            return 'china';
        }
        
        return 'global';
    } catch (error) {
        console.warn('Failed to detect region, defaulting to global:', error.message);
        return 'global';
    }
}

// Scrape ImageMagick website for portable versions
async function scrapeImageMagickVersions(baseUrl) {
    console.log(`🔍 Scraping ImageMagick versions from: ${baseUrl}`);
    
    try {
        const response = http.get(baseUrl);
        if (response.error) {
            throw new Error(`HTTP request failed: ${response.error}`);
        }
        
        console.log(`📊 Response status: ${response.status_code}`);
        console.log(`📄 Response body length: ${response.body.length}`);
        
        const html = response.body;
        
        // Debug: Show first 500 characters of HTML
        console.log(`📝 HTML preview: ${html.substring(0, 500)}`);
        
        // Debug: Look for any ZIP files in the HTML
        console.log(`🔍 Looking for any ZIP files...`);
        const zipPattern = /href="([^"]*\.zip)"/g;
        let zipMatches = [];
        let zipMatch;
        while ((zipMatch = zipPattern.exec(html)) !== null) {
            zipMatches.push(zipMatch[1]);
        }
        console.log(`📦 All ZIP files found: ${zipMatches.join(', ')}`);
        
        // Debug: Look for any files containing "ImageMagick"
        console.log(`🔍 Looking for files with "ImageMagick" in name...`);
        const imagemagickPattern = /href="([^"]*ImageMagick[^"]*)"/g;
        let imagemagickMatches = [];
        let imagemagickMatch;
        while ((imagemagickMatch = imagemagickPattern.exec(html)) !== null) {
            imagemagickMatches.push(imagemagickMatch[1]);
        }
        console.log(`🎯 ImageMagick files found: ${imagemagickMatches.join(', ')}`);
        
        // Parse HTML to find portable archive files (7z or zip)
        const patterns = [
            /href="(ImageMagick-([0-9.]+-[0-9]+)-portable-Q16-x64\.7z)"/g,
            /href="(ImageMagick-([0-9.]+)-portable-Q16-x64\.7z)"/g,
            /href="(ImageMagick-([0-9.]+)-[0-9]+-portable-Q16-x64\.7z)"/g,
            /href="([^"]*?ImageMagick[^"]*?portable[^"]*?\.(7z|zip))"/g,
            /href="([^"]*?ImageMagick[^"]*?\.(7z|zip))"/g  // More general pattern
        ];
        
        const versions = [];
        
        for (let patternIndex = 0; patternIndex < patterns.length; patternIndex++) {
            const pattern = patterns[patternIndex];
            console.log(`🔍 Trying pattern ${patternIndex + 1}: ${pattern}`);
            
            let match;
            let matchCount = 0;
            
            while ((match = pattern.exec(html)) !== null) {
                console.log(`🎯 Pattern ${patternIndex + 1} found match: ${match[1]}`);
                if (match.length > 2) {
                    console.log(`   Version: ${match[2]}`);
                }
                
                versions.push({
                    filename: match[1],
                    version: match[2] || 'unknown',
                    url: baseUrl + match[1],
                    timestamp: Date.now(),
                    pattern: patternIndex + 1
                });
                matchCount++;
            }
            
            console.log(`📋 Pattern ${patternIndex + 1} matches: ${matchCount}`);
            
            if (matchCount > 0) {
                break; // Use the first pattern that finds matches
            }
        }
        
        // Sort by version (newest first)
        versions.sort((a, b) => {
            const aParts = a.version.split(/[-.]/);
            const bParts = b.version.split(/[-.]/);
            
            for (let i = 0; i < Math.max(aParts.length, bParts.length); i++) {
                const aPart = parseInt(aParts[i]) || 0;
                const bPart = parseInt(bParts[i]) || 0;
                
                if (aPart !== bPart) {
                    return bPart - aPart; // Descending order
                }
            }
            
            return 0;
        });
        
        console.log(`📋 Found ${versions.length} portable versions`);
        return versions;
        
    } catch (error) {
        console.error(`❌ Failed to scrape ImageMagick versions: ${error.message}`);
        throw error;
    }
}

// Get the latest portable version URL based on region
async function getLatestPortableUrl(region) {
    console.log(`🌍 Detected region: ${region}`);
    
    let versions = [];
    let usedMirror = false;
    
    // Try mirror first for China region
    if (region === 'china') {
        try {
            // Try to get from mirror (might be pre-cached)
            const mirrorResponse = http.get(MIRROR_BASE_URL);
            if (!mirrorResponse.error) {
                const mirrorHtml = mirrorResponse.body;
                const mirrorPattern = /href="(ImageMagick-([0-9.]+-[0-9]+)-portable-Q16-x64\.7z)"/g;
                let match;
                
                while ((match = mirrorPattern.exec(mirrorHtml)) !== null) {
                    versions.push({
                        filename: match[1],
                        version: match[2],
                        url: MIRROR_BASE_URL + match[1],
                        timestamp: Date.now(),
                        source: 'mirror'
                    });
                }
                
                if (versions.length > 0) {
                    usedMirror = true;
                    console.log(`✅ Found ${versions.length} versions on mirror`);
                }
            }
        } catch (error) {
            console.warn(`⚠️  Mirror access failed, trying official site: ${error.message}`);
        }
    }
    
    // If no versions from mirror or not China region, scrape official site
    if (versions.length === 0) {
        try {
            const officialVersions = await scrapeImageMagickVersions(IMAGEMAGICK_BASE_URL);
            versions = officialVersions.map(v => ({ ...v, source: 'official' }));
        } catch (error) {
            throw new Error(`Failed to get ImageMagick versions from both mirror and official site: ${error.message}`);
        }
    }
    
    if (versions.length === 0) {
        throw new Error('No portable ImageMagick versions found');
    }
    
    const latest = versions[0];
    console.log(`🎯 Selected version ${latest.version} from ${latest.source}: ${latest.filename}`);
    
    return {
        url: latest.url,
        version: latest.version,
        filename: latest.filename,
        source: latest.source
    };
}

// Download and extract ImageMagick (moved logic to main function for persistent downloads)

// Validate installation by checking executables
async function validateInstallation(installInfo) {
    const { extractDir, executables } = installInfo;
    
    console.log(`🔍 Validating ImageMagick installation...`);
    
    const results = {};
    let mainExecutable = null;
    
    for (const exe of executables) {
        const exePath = fs.join([extractDir, exe]);
        
        try {
            const stats = await fs.stat(exePath);
            if (stats.isFile()) {
                results[exe] = {
                    exists: true,
                    path: exePath,
                    size: stats.size
                };
                
                // Try to get version from the main executable
                if (exe === 'magick.exe' && !mainExecutable) {
                    mainExecutable = exePath;
                }
                
                console.log(`✅ Found ${exe} (${Math.round(stats.size / 1024)}KB)`);
            } else {
                results[exe] = { exists: false, error: 'Not a file' };
            }
        } catch (error) {
            results[exe] = { exists: false, error: error.message };
            console.log(`❌ ${exe}: ${error.message}`);
        }
    }
    
    // Test version command if main executable found
    if (mainExecutable) {
        try {
            console.log(`🧪 Testing version command...`);
            const result = await exec(mainExecutable, ['-version']);
            
            if (result.exitCode === 0) {
                const output = result.stdout || result.stderr || '';
                const versionMatch = output.match(/Version: ImageMagick ([^\s]+)/);
                
                if (versionMatch) {
                    console.log(`✅ Version check passed: ImageMagick ${versionMatch[1]}`);
                    results.version = versionMatch[1];
                } else {
                    console.log(`⚠️  Version pattern not found in output`);
                    results.version = 'unknown';
                }
            } else {
                console.log(`⚠️  Version command failed with exit code ${result.exitCode}`);
                results.version = 'unknown';
            }
        } catch (error) {
            console.log(`⚠️  Version command test failed: ${error.message}`);
            results.version = 'unknown';
        }
    }
    
    // Check if at least one executable exists
    const hasExecutables = Object.values(results).some(r => r.exists);
    
    return {
        success: hasExecutables,
        results: results,
        mainExecutable: mainExecutable,
        extractDir: extractDir
    };
}

// Set up PATH and create symlinks
async function setupEnvironment(installInfo, toolsDir) {
    const { extractDir, mainExecutable, results } = installInfo;
    
    console.log(`🔧 Setting up environment...`);
    
    // Create tools directory if it doesn't exist
    await fs.mkdir(toolsDir);
    
    // Create symlinks for main executables in tools directory
    const symlinks = [];
    
    for (const [exe, info] of Object.entries(results)) {
        if (info.exists && info.path) {
            const targetPath = fs.join([toolsDir, exe]);
            
            try {
                // Remove existing symlink if it exists
                try {
                    await fs.remove(targetPath);
                } catch (error) {
                    // Ignore if it doesn't exist
                }
                
                // Note: symlink not available in workflow engine
                // Copy file instead
                await fs.copy(info.path, targetPath);
                symlinks.push({ from: info.path, to: targetPath });
                console.log(`📋 Copied: ${exe} -> ${info.path}`);
                
            } catch (error) {
                console.warn(`⚠️  Failed to create symlink for ${exe}: ${error.message}`);
            }
        }
    }
    
    // Return main executable path for PATH setup
    const mainTarget = mainExecutable ? fs.join([toolsDir, 'magick.exe']) : null;
    
    return {
        symlinks: symlinks,
        mainExecutable: mainTarget,
        toolsDir: toolsDir
    };
}

// Main workflow execution
async function main() {
    console.log(`🚀 Starting ${WORKFLOW_NAME}`);
    console.log('=' .repeat(50));
    
    try {
        // Get configuration from environment variables
        const homeDir = getVar('HOME') || '/tmp';
        const installDir = getVar('INSTALL_DIR') || fs.join([homeDir, '.amo', 'tools']);
        const toolsDir = getVar('TOOLS_DIR') || fs.join([homeDir, '.amo', 'bin']);
        const downloadsDir = getVar('DOWNLOADS_DIR') || fs.join([homeDir, '.amo', 'downloads']);
        
        console.log(`📁 Install directory: ${installDir}`);
        console.log(`📁 Tools directory: ${toolsDir}`);
        console.log(`📁 Downloads directory: ${downloadsDir}`);
        
        // Create downloads directory if it doesn't exist
        await fs.mkdir(downloadsDir);
        
        // Detect region
        const region = detectRegion();
        
        // Get latest portable URL
        const downloadInfo = await getLatestPortableUrl(region);
        
        // Download to persistent downloads directory first
        const downloadPath = fs.join([downloadsDir, downloadInfo.filename]);
        console.log(`📥 Downloading to persistent location: ${downloadPath}`);
        
        // Download using http.downloadFileResume (with breakpoint resume support)
        const downloadResult = http.downloadFileResume(downloadInfo.url, downloadPath, { show_progress: true });
        
        if (downloadResult.error) {
            throw new Error(`Download failed: ${downloadResult.error}`);
        }
        
        if (downloadResult.status_code !== 200 && downloadResult.status_code !== 206) {
            throw new Error(`Download failed: HTTP ${downloadResult.status_code}`);
        }
        
        console.log('\n✅ Download completed to persistent location');
        
        // Now copy to install directory for extraction
        const tempZipPath = fs.join([installDir, downloadInfo.filename]);
        console.log(`📋 Copying from downloads to install directory...`);
        await fs.copy(downloadPath, tempZipPath);
        
        // Extract 7z file
        console.log(`📦 Extracting ${downloadInfo.filename}...`);
        
        // Create extraction directory
        const extractDir = fs.join([installDir, `imagemagick-${downloadInfo.version}`]);
        await fs.mkdir(extractDir);
        
        // Extract 7z using external command
        console.log(`📦 Extracting 7z file...`);
        const extractResult = await cliCommand('7z', ['x', tempZipPath, '-o' + extractDir, '-y'], {
            timeout: 300,
            cwd: extractDir
        });
        
        if (extractResult.error) {
            throw new Error(`7z extraction failed: ${extractResult.error}`);
        }
        
        console.log(`✅ 7z extraction completed`);
        
        console.log(`✅ Extraction completed to: ${extractDir}`);
        
        // Clean up temporary file from install directory (keep persistent download)
        await fs.remove(tempZipPath);
        console.log(`🧹 Cleaned up temporary file: ${downloadInfo.filename}`);
        
        const extractInfo = {
            extractDir: extractDir,
            version: downloadInfo.version,
            executables: ['magick.exe', 'convert.exe', 'identify.exe', 'mogrify.exe']
        };
        
        // Validate installation
        const validation = await validateInstallation(extractInfo);
        
        if (!validation.success) {
            throw new Error('ImageMagick installation validation failed');
        }
        
        // Set up environment
        const setup = await setupEnvironment(validation, toolsDir);
        
        // Return results
        const result = {
            success: true,
            version: validation.results.version || downloadInfo.version,
            installPath: validation.extractDir,
            toolsPath: setup.toolsDir,
            mainExecutable: setup.mainExecutable,
            region: region,
            source: downloadInfo.source,
            symlinks: setup.symlinks.length
        };
        
        console.log('\n✅ ImageMagick installation completed successfully!');
        console.log(`   Version: ${result.version}`);
        console.log(`   Install Path: ${result.installPath}`);
        console.log(`   Tools Path: ${result.toolsPath}`);
        console.log(`   Main Executable: ${result.mainExecutable}`);
        console.log(`   Symlinks Created: ${result.symlinks}`);
        console.log(`   Source: ${result.source} (${result.region})`);
        
        return result;
        
    } catch (error) {
        console.error(`\n❌ ${WORKFLOW_NAME} failed: ${error.message}`);
        
        return {
            success: false,
            error: error.message,
            workflow: WORKFLOW_NAME
        };
    }
}

// Execute workflow
main().then(result => {
    process.exit(result.success ? 0 : 1);
}).catch(error => {
    console.error('Workflow execution error:', error);
    process.exit(1);
});