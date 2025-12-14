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

const REGION_MIRRORS = {
    'cn': 'https://toolchains.mirror.toulan.fun/',
    'china': 'https://toolchains.mirror.toulan.fun/',
};

function getMirrorUrl(region) {
    const normalizedRegion = region.toLowerCase();
    return REGION_MIRRORS[normalizedRegion] || null;
}

function getWindowsArch() {
    const candidates = [
        getVar('ARCH'),
        getVar('arch'),
        getVar('PROCESSOR_ARCHITECTURE'),
        getVar('PROCESSOR_ARCHITEW6432'),
    ];
    let arch = '';
    for (let i = 0; i < candidates.length; i++) {
        if (candidates[i]) {
            arch = String(candidates[i]).toLowerCase();
            if (arch) {
                break;
            }
        }
    }
    if (!arch && typeof process !== 'undefined' && process.arch) {
        arch = String(process.arch).toLowerCase();
    }
    if (arch.indexOf('64') !== -1 || arch.indexOf('x64') !== -1 || arch.indexOf('amd64') !== -1) {
        return 'x64';
    }
    if (arch.indexOf('86') !== -1 || arch.indexOf('32') !== -1 || arch.indexOf('ia32') !== -1) {
        return 'x86';
    }
    return 'x64';
}

function parseVersionFromFilename(filename) {
    const match = filename.match(/ImageMagick-([0-9.]+-[0-9]+)-portable/);
    if (match && match[1]) {
        return match[1];
    }
    return 'unknown';
}

function detectArchFromFilename(filename) {
    const lower = filename.toLowerCase();
    if (lower.indexOf('x64') !== -1 || lower.indexOf('x86_64') !== -1 || lower.indexOf('win64') !== -1 || lower.indexOf('amd64') !== -1) {
        return 'x64';
    }
    if (lower.indexOf('x86') !== -1 || lower.indexOf('win32') !== -1 || lower.indexOf('i386') !== -1) {
        return 'x86';
    }
    return null;
}

function sortVersionsDescending(versions) {
    versions.sort((a, b) => {
        const aParts = String(a.version || '').split(/[-.]/);
        const bParts = String(b.version || '').split(/[-.]/);
        for (let i = 0; i < Math.max(aParts.length, bParts.length); i++) {
            const aPart = parseInt(aParts[i]) || 0;
            const bPart = parseInt(bParts[i]) || 0;
            if (aPart !== bPart) {
                return bPart - aPart;
            }
        }
        return 0;
    });
    return versions;
}

function filterVersionsByArch(versions, arch) {
    const targetArch = arch === 'x86' ? 'x86' : 'x64';
    const exactMatches = versions.filter(v => v.arch === targetArch);
    if (exactMatches.length > 0) {
        return exactMatches;
    }
    if (targetArch === 'x64') {
        const nonX86 = versions.filter(v => v.arch !== 'x86');
        if (nonX86.length > 0) {
            return nonX86;
        }
    }
    return versions;
}

async function scrapeImageMagickVersions(baseUrl, arch) {
    console.log(`🔍 Scraping ImageMagick versions from: ${baseUrl}`);
    
    try {
        const response = http.get(baseUrl);
        if (response.error) {
            throw new Error(`HTTP request failed: ${response.error}`);
        }
        
        console.log(`📊 Response status: ${response.status_code}`);
        console.log(`📄 Response body length: ${response.body.length}`);
        
        const html = response.body;
        
        console.log(` Looking for any ZIP files...`);
        const zipPattern = /href="([^"]*\.zip)"/g;
        let zipMatches = [];
        let zipMatch;
        while ((zipMatch = zipPattern.exec(html)) !== null) {
            zipMatches.push(zipMatch[1]);
        }
        console.log(`📦 All ZIP files found: ${zipMatches.join(', ')}`);
        
        const imagemagickPattern = /href="([^"]*ImageMagick[^"]*)"/g;
        let imagemagickMatches = [];
        let imagemagickMatch;
        while ((imagemagickMatch = imagemagickPattern.exec(html)) !== null) {
            imagemagickMatches.push(imagemagickMatch[1]);
        }
        console.log(`🎯 Found ${imagemagickMatches.length} ImageMagick files`);
        
        const versions = [];
        const portablePattern = /href="([^"]*ImageMagick[^"]*portable[^"]*\.(7z|zip))"/g;
        let match;
        let matchCount = 0;
        while ((match = portablePattern.exec(html)) !== null) {
            const filename = match[1];
            const version = parseVersionFromFilename(filename);
            const fileArch = detectArchFromFilename(filename);
            versions.push({
                filename: filename,
                version: version,
                url: baseUrl + filename,
                timestamp: Date.now(),
                pattern: 1,
                arch: fileArch
            });
            matchCount++;
        }
        
        console.log(`📋 Portable matches: ${matchCount}`);
        
        const filtered = filterVersionsByArch(versions, arch);
        sortVersionsDescending(filtered);
        
        console.log(`📋 Found ${filtered.length} portable versions after architecture filter`);
        return filtered;
        
    } catch (error) {
        console.error(`❌ Failed to scrape ImageMagick versions: ${error.message}`);
        throw error;
    }
}

async function getLatestPortableUrl(region, arch) {
    console.log(`🌍 Detected region: ${region}`);
    
    let versions = [];
    let usedMirror = false;
    
    const mirrorUrl = getMirrorUrl(region);
    
    if (mirrorUrl) {
        console.log(`🔄 Mirror found for region ${region}, trying mirror first: ${mirrorUrl}`);
        try {
            const versionsJsonUrl = mirrorUrl + 'versions.json';
            console.log(`📡 Fetching versions.json from: ${versionsJsonUrl}`);
            
            const versionsResponse = http.get(versionsJsonUrl);
            if (versionsResponse.error) {
                throw new Error(`Failed to fetch versions.json: ${versionsResponse.error}`);
            }
            
            const versionsData = JSON.parse(versionsResponse.body);
            
            if (versionsData.software && versionsData.software.files) {
                console.log(`📦 Found software section with ${versionsData.software.files.length} files`);
                
                for (const file of versionsData.software.files) {
                    if (file.name.includes('ImageMagick') && file.name.includes('portable') && (file.name.endsWith('.zip') || file.name.endsWith('.7z'))) {
                        const version = parseVersionFromFilename(file.name);
                        const fileArch = detectArchFromFilename(file.name);
                        versions.push({
                            filename: file.name,
                            version: version,
                            url: mirrorUrl + 'software/' + file.name,
                            size: file.size,
                            timestamp: Date.now(),
                            source: 'mirror',
                            arch: fileArch
                        });
                        
                        console.log(`🎯 Found ImageMagick on mirror: ${file.name} (${version})`);
                    }
                }
                
                if (versions.length > 0) {
                    usedMirror = true;
                    console.log(`✅ Found ${versions.length} ImageMagick versions on mirror before architecture filter`);
                    
                    versions = filterVersionsByArch(versions, arch);
                    sortVersionsDescending(versions);
                    console.log(`📋 Using ${versions.length} ImageMagick versions on mirror after architecture filter`);
                } else {
                    console.log(`⚠️  No ImageMagick portable versions found in mirror software section`);
                }
            } else {
                console.log(`⚠️  No software section found in versions.json`);
            }
        } catch (error) {
            console.warn(`⚠️  Mirror access failed, trying official site: ${error.message}`);
        }
    } else {
        console.log(`🌍 No mirror configured for region ${region}, using official site`);
    }
    
    if (versions.length === 0) {
        try {
            console.log(`📡 Scraping official ImageMagick site...`);
            const officialVersions = await scrapeImageMagickVersions(IMAGEMAGICK_BASE_URL, arch);
            versions = officialVersions.map(v => ({ ...v, source: 'official' }));
        } catch (error) {
            throw new Error(`Failed to get ImageMagick versions from official site: ${error.message}`);
        }
    }
    
    if (versions.length === 0) {
        throw new Error('No portable ImageMagick versions found');
    }
    
    let latest = versions[0];
    
    const zipVersions = versions.filter(v => v.filename && v.filename.toLowerCase().endsWith('.zip'));
    if (zipVersions.length > 0) {
        latest = zipVersions[0];
    }
    
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
    let versionCommandSucceeded = false;
    
    for (const exe of executables) {
        const exePath = fs.join([extractDir, exe]);
        
        try {
            const stats = await fs.stat(exePath);
            if (stats && stats.success && stats.data && !stats.data.is_dir) {
                results[exe] = {
                    exists: true,
                    path: exePath,
                    size: stats.data.size
                };
                
                // Try to get version from the main executable
                if (exe === 'magick.exe' && !mainExecutable) {
                    mainExecutable = exePath;
                }
                
                console.log(`✅ Found ${exe} (${Math.round(stats.data.size / 1024)}KB)`);
            } else {
                const findResult = fs.find(extractDir, exe);
                if (findResult && findResult.success && Array.isArray(findResult.files) && findResult.files.length > 0) {
                    const foundPath = findResult.files[0];
                    const foundStats = await fs.stat(foundPath);
                    if (foundStats && foundStats.success && foundStats.data && !foundStats.data.is_dir) {
                        results[exe] = {
                            exists: true,
                            path: foundPath,
                            size: foundStats.data.size
                        };
                        if (exe === 'magick.exe' && !mainExecutable) {
                            mainExecutable = foundPath;
                        }
                        console.log(`✅ Found ${exe} (${Math.round(foundStats.data.size / 1024)}KB)`);
                    } else {
                        results[exe] = { exists: false, error: 'Found path invalid' };
                    }
                } else {
                    results[exe] = { exists: false, error: 'Not a file' };
                }
            }
        } catch (error) {
            const findResult = fs.find(extractDir, exe);
            if (findResult && findResult.success && Array.isArray(findResult.files) && findResult.files.length > 0) {
                const foundPath = findResult.files[0];
                const foundStats = await fs.stat(foundPath);
                if (foundStats && foundStats.success && foundStats.data && !foundStats.data.is_dir) {
                    results[exe] = {
                        exists: true,
                        path: foundPath,
                        size: foundStats.data.size
                    };
                    if (exe === 'magick.exe' && !mainExecutable) {
                        mainExecutable = foundPath;
                    }
                    console.log(`✅ Found ${exe} (${Math.round(foundStats.data.size / 1024)}KB)`);
                } else {
                    results[exe] = { exists: false, error: error.message };
                    console.log(`❌ ${exe}: ${error.message}`);
                }
            } else {
                results[exe] = { exists: false, error: error.message };
                console.log(`❌ ${exe}: ${error.message}`);
            }
        }
    }
    
    // Test version command if main executable found
    if (mainExecutable) {
        try {
            console.log(`🧪 Testing version command...`);
            const result = await cliCommand(mainExecutable, ['-version']);
            
            if (!result.error) {
                versionCommandSucceeded = true;
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
                console.log(`⚠️  Version command failed: ${result.error}`);
                results.version = 'unknown';
            }
        } catch (error) {
            console.log(`⚠️  Version command test failed: ${error.message}`);
            results.version = 'unknown';
        }
    }
    
    const hasExecutables = Object.values(results).some(r => r && r.exists);
    const success = hasExecutables && (!mainExecutable || versionCommandSucceeded);
    
    return {
        success: success,
        results: results,
        mainExecutable: mainExecutable,
        extractDir: extractDir
    };
}

// Set up PATH and create symlinks
async function setupEnvironment(installInfo, toolsDir) {
    const { extractDir, mainExecutable, results } = installInfo;
    
    console.log(`🔧 Setting up environment...`);
    
    await fs.mkdir(toolsDir);
    
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
        // Check if running on Windows
        const osType = getOS();
        if (osType !== 'windows') {
            console.error(`❌ This installer is designed for Windows systems only.`);
            console.error(`   Current OS: ${osType}`);
            console.error(`   Please use the appropriate installer for your operating system.`);
            throw new Error(`Unsupported operating system: ${osType}`);
        }
        console.log(`✅ Running on Windows system`);
        
        const providedInstallDir = getVar('installDir');
        const homeDir = getVar('HOME') || getVar('USERPROFILE') || providedInstallDir || '/tmp';
        const installDir = getVar('INSTALL_DIR') || providedInstallDir || fs.join([homeDir, '.amo', 'tools']);
        const toolsDir = getVar('TOOLS_DIR') || installDir;
        const downloadsDir = getVar('DOWNLOADS_DIR') || fs.join([homeDir, '.amo', 'downloads']);
        
        console.log(`📁 Install directory: ${installDir}`);
        console.log(`📁 Tools directory: ${toolsDir}`);
        console.log(`📁 Downloads directory: ${downloadsDir}`);
        
        await fs.mkdir(downloadsDir);
        
        const region = getRegion();
        console.log(`🌍 Detected region: ${region}`);
        const arch = getWindowsArch();
        console.log(`🖥 Detected architecture: ${arch}`);
        const downloadInfo = await getLatestPortableUrl(region, arch);
        
        // Download to persistent downloads directory first
        const downloadPath = fs.join([downloadsDir, downloadInfo.filename]);
        console.log(`📥 Downloading to persistent location: ${downloadPath}`);
        
        // Check if file already exists
        let downloadResult;
        try {
            const stats = await fs.stat(downloadPath);
            console.log(`🔍 File stats:`, JSON.stringify(stats));
            // Check if it's a file and has content
            if (stats && stats.success && stats.data && !stats.data.is_dir && stats.data.size > 0) {
                console.log(`✅ File already exists, skipping download (${Math.round(stats.data.size / 1024 / 1024)}MB)`);
                downloadResult = { status_code: 200, error: null };
            } else {
                throw new Error("File exists but is empty or not a regular file");
            }
        } catch (error) {
            // File doesn't exist or is empty, download it
            console.log(`📥 File not found, starting download... (${error.message})`);
            downloadResult = http.downloadFileResume(downloadInfo.url, downloadPath, { show_progress: true });
            
            if (downloadResult.error) {
                throw new Error(`Download failed: ${downloadResult.error}`);
            }
            
            if (downloadResult.status_code !== 200 && downloadResult.status_code !== 206) {
                throw new Error(`Download failed: HTTP ${downloadResult.status_code}`);
            }
            
            console.log('\n✅ Download completed to persistent location');
        }
        
        // Now copy to install directory for extraction
        const tempZipPath = fs.join([installDir, downloadInfo.filename]);
        console.log(`📋 Copying from downloads to install directory...`);
        await fs.copy(downloadPath, tempZipPath);
        
        const isZip = downloadInfo.filename.toLowerCase().endsWith('.zip');
        
        console.log(`📦 Extracting ${downloadInfo.filename}...`);
        
        // Create extraction directory
        const extractDir = fs.join([installDir, `imagemagick-${downloadInfo.version}`]);
        
        // Check if already extracted
        try {
            const stats = await fs.stat(extractDir);
            console.log(`🔍 Directory stats:`, JSON.stringify(stats));
            // Check if it's a directory
            if (stats && stats.success && stats.data && stats.data.is_dir) {
                console.log(`✅ Already extracted to: ${extractDir}`);
            } else {
                throw new Error("Path exists but is not a directory");
            }
        } catch (error) {
            // Directory doesn't exist, extract the file
            console.log(`📦 Directory not found, extracting files... (${error.message})`);
            await fs.mkdir(extractDir);
            
            let extractionSucceeded = false;
            
            if (isZip) {
                console.log(`📦 Using built-in ZIP extractor...`);
                const extractResult = fs.extractZip(tempZipPath, extractDir);
                if (!extractResult || !extractResult.success) {
                    const errorMessage = extractResult && extractResult.error ? extractResult.error : 'unknown error';
                    throw new Error(`ZIP extraction failed: ${errorMessage}`);
                }
                extractionSucceeded = true;
            } else {
                console.log(`📦 Extracting 7z file...`);
                const extractResult = await cliCommand('7z', ['x', tempZipPath, '-o' + extractDir, '-y'], {
                    timeout: 300,
                    cwd: extractDir
                });
                
                if (extractResult.error) {
                    throw new Error(`7z extraction failed: ${extractResult.error}`);
                }
                
                extractionSucceeded = true;
            }
            
            if (extractionSucceeded) {
                console.log(`✅ Extraction completed to: ${extractDir}`);
                
                await fs.remove(tempZipPath);
                console.log(`🧹 Cleaned up temporary file: ${downloadInfo.filename}`);
            }
        }
        
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
