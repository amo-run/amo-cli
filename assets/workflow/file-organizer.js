//!amo

// File Organizer - Organize files by extension into subdirectories
// Usage: amo run file-organizer.js --var input=/path/to/messy/folder --var output=/path/to/organized/folder

function main() {
    console.log("📂 File Organizer");
    console.log("==================");

    // Get configuration from runtime variables
    var optHelp = getVar("help") === "true"
    var sourceDir = getVar("input") || "";
    var targetDir = getVar("output") || "";
    var dryRun = getVar("dry_run") === "true";
    var copyMode = getVar("copy") === "true"; // Copy instead of move
    var includeHidden = getVar("include_hidden") === "true";
    var overwrite = getVar("overwrite") === "true";

    // Show help message
    if (optHelp) {
        console.log("Organize files by extension into subdirectories");
        console.log("");
        console.log("Supported variables:");
        console.log("  --var help=true: Show help message");
        console.log("  --var input=/path/to/messy/folder: Source directory");
        console.log("  --var output=/path/to/organized/folder: Target directory");
        console.log("  --var dry_run=true: Dry run mode (no changes will be made)");
        console.log("  --var copy=true: Copy mode (copy files instead of moving)");
        console.log("  --var include_hidden=true: Include hidden files");
        console.log("  --var overwrite=true: Overwrite existing files");
        console.log("");
        console.log("Examples:");
        console.log("  amo run file-organizer.js --input /Downloads --output /Organized");
        console.log("  amo run file-organizer.js --var input=/Downloads --var output=/Organized --var dry_run=true");
        console.log("  amo run file-organizer.js --var input=/Downloads --var output=/Organized --var copy=true");

        return true;
    }

    console.log("📁 Source directory:", sourceDir || "Not specified");
    console.log("📁 Target directory:", targetDir || "Not specified");
    console.log("🔄 Mode:", copyMode ? "Copy" : "Move");
    console.log("👁️ Include hidden files:", includeHidden ? "Yes" : "No");
    console.log("🔄 Overwrite existing:", overwrite ? "Yes" : "No");
    console.log("🧪 Dry run:", dryRun ? "Yes (no changes will be made)" : "No");
    console.log("");

    // Validate required parameters
    if (!sourceDir) {
        console.error("❌ Error: input is required");
        console.log("Usage: --var input=/path/to/source");
        console.log("   or: --input /path/to/source");
        return false;
    }

    if (!targetDir) {
        console.error("❌ Error: output is required");
        console.log("Usage: --var output=/path/to/target");
        console.log("   or: --output /path/to/target");
        return false;
    }

    // Validate source directory exists
    if (!fs.exists(sourceDir)) {
        console.error("❌ Error: Source directory does not exist:", sourceDir);
        return false;
    }

    if (!fs.isDir(sourceDir)) {
        console.error("❌ Error: Source path is not a directory:", sourceDir);
        return false;
    }

    // Create target directory if it doesn't exist
    if (!fs.exists(targetDir)) {
        console.log("📁 Creating target directory:", targetDir);
        if (!dryRun) {
            var createResult = fs.mkdir(targetDir);
            if (!createResult.success) {
                console.error("❌ Failed to create target directory:", createResult.error);
                return false;
            }
        }
    }

    // Get list of files in source directory
    console.log("🔍 Scanning source directory...");
    var listResult = fs.readdir(sourceDir);
    if (!listResult.success) {
        console.error("❌ Failed to list source directory:", listResult.error);
        return false;
    }

    // Filter files (exclude directories and optionally hidden files)
    var files = [];
    for (var i = 0; i < listResult.files.length; i++) {
        var file = listResult.files[i];
        
        // Safety check: ensure file object has required properties
        if (!file || typeof file.Name !== 'string') {
            console.warn("⚠️  Skipping invalid file entry:", file);
            continue;
        }
        
        // Skip directories
        if (file.IsDir) {
            continue;
        }

        // Skip hidden files if not included
        if (!includeHidden && file.Name.startsWith(".")) {
            continue;
        }

        files.push(file);
    }

    console.log("📊 Found", files.length, "files to organize");
    console.log("");

    if (files.length === 0) {
        console.log("ℹ️  No files to organize");
        return true;
    }

    // Group files by extension
    var extensionGroups = {};
    var noExtensionFiles = [];

    for (var i = 0; i < files.length; i++) {
        var file = files[i];
        var extension = fs.ext(file.Name).toLowerCase();
        
        if (extension === "") {
            noExtensionFiles.push(file);
        } else {
            // Remove the dot from extension
            extension = extension.substring(1);
            
            if (!extensionGroups[extension]) {
                extensionGroups[extension] = [];
            }
            extensionGroups[extension].push(file);
        }
    }

    // Display organization plan
    console.log("📋 Organization Plan:");
    console.log("---------------------");

    var totalFiles = 0;
    for (var ext in extensionGroups) {
        var count = extensionGroups[ext].length;
        totalFiles += count;
        console.log("📁 " + ext.toUpperCase() + " files: " + count + " files → " + fs.join([targetDir, ext]));
    }

    if (noExtensionFiles.length > 0) {
        totalFiles += noExtensionFiles.length;
        console.log("📁 No extension: " + noExtensionFiles.length + " files → " + fs.join([targetDir, "no_extension"]));
    }

    console.log("📊 Total files to organize:", totalFiles);
    console.log("");

    if (dryRun) {
        console.log("🧪 Dry run mode - no changes will be made");
        return true;
    }

    // Organize files by extension
    var successCount = 0;
    var errorCount = 0;

    // Process files with extensions
    for (var ext in extensionGroups) {
        var extFiles = extensionGroups[ext];
        var extDir = fs.join([targetDir, ext]);

        console.log("📁 Processing " + ext.toUpperCase() + " files (" + extFiles.length + " files)...");

        // Create extension directory
        var createDirResult = fs.mkdir(extDir);
        if (!createDirResult.success) {
            console.error("❌ Failed to create directory " + extDir + ":", createDirResult.error);
            errorCount += extFiles.length;
            continue;
        }

        // Move/copy files
        for (var i = 0; i < extFiles.length; i++) {
            var file = extFiles[i];
            var sourcePath = file.Path;
            var targetPath = fs.join([extDir, file.Name]);

            // Handle file name conflicts
            if (!overwrite) {
                var counter = 1;
                var originalTargetPath = targetPath;
                while (fs.exists(targetPath)) {
                    var baseName = fs.basename(file.Name);
                    var extension = fs.ext(file.Name);
                    var newName = baseName + "_" + counter + extension;
                    targetPath = fs.join([extDir, newName]);
                    counter++;
                }

                if (targetPath !== originalTargetPath) {
                    console.log("⚠️  File name conflict, renaming to:", fs.basename(targetPath));
                }
            }

            // Perform the operation
            var result;
            if (copyMode) {
                result = fs.copy(sourcePath, targetPath);
            } else {
                result = fs.move(sourcePath, targetPath);
            }

            if (!result.success) {
                console.error("❌ Failed to " + (copyMode ? "copy" : "move") + " " + file.Name + ":", result.error);
                errorCount++;
            } else {
                console.log("✅ " + (copyMode ? "Copied" : "Moved") + ": " + file.Name);
                successCount++;
            }
        }
    }

    // Process files without extensions
    if (noExtensionFiles.length > 0) {
        var noExtDir = fs.join([targetDir, "no_extension"]);
        
        console.log("📁 Processing files without extensions (" + noExtensionFiles.length + " files)...");

        // Create no_extension directory
        var createDirResult = fs.mkdir(noExtDir);
        if (!createDirResult.success) {
            console.error("❌ Failed to create directory " + noExtDir + ":", createDirResult.error);
            errorCount += noExtensionFiles.length;
        } else {
            // Move/copy files
            for (var i = 0; i < noExtensionFiles.length; i++) {
                var file = noExtensionFiles[i];
                var sourcePath = file.Path;
                var targetPath = fs.join([noExtDir, file.Name]);

                // Handle file name conflicts
                if (!overwrite) {
                    var counter = 1;
                    var originalTargetPath = targetPath;
                    while (fs.exists(targetPath)) {
                        var newName = file.Name + "_" + counter;
                        targetPath = fs.join([noExtDir, newName]);
                        counter++;
                    }

                    if (targetPath !== originalTargetPath) {
                        console.log("⚠️  File name conflict, renaming to:", fs.basename(targetPath));
                    }
                }

                // Perform the operation
                var result;
                if (copyMode) {
                    result = fs.copy(sourcePath, targetPath);
                } else {
                    result = fs.move(sourcePath, targetPath);
                }

                if (!result.success) {
                    console.error("❌ Failed to " + (copyMode ? "copy" : "move") + " " + file.Name + ":", result.error);
                    errorCount++;
                } else {
                    console.log("✅ " + (copyMode ? "Copied" : "Moved") + ": " + file.Name);
                    successCount++;
                }
            }
        }
    }

    // Summary
    console.log("");
    console.log("📊 Organization Summary:");
    console.log("========================");
    console.log("✅ Successfully processed:", successCount, "files");
    console.log("❌ Errors:", errorCount, "files");
    console.log("📁 Target directory:", targetDir);

    if (successCount > 0) {
        console.log("");
        console.log("🎉 File organization completed!");
        
        // Show final directory structure
        console.log("");
        console.log("📂 Final directory structure:");
        showDirectoryStructure(targetDir, 0);
    }

    return errorCount === 0;
}

// Helper function to show directory structure
function showDirectoryStructure(dirPath, depth) {
    var indent = "";
    for (var i = 0; i < depth; i++) {
        indent += "  ";
    }

    var listResult = fs.readdir(dirPath);
    if (!listResult.success) {
        console.log(indent + "❌ Error reading directory:", listResult.error);
        return;
    }

    // Sort files and directories
    var dirs = [];
    var files = [];
    
    for (var i = 0; i < listResult.files.length; i++) {
        var item = listResult.files[i];
        if (!item || typeof item.Name !== 'string') {
            continue;
        }
        
        if (item.IsDir) {
            dirs.push(item);
        } else {
            files.push(item);
        }
    }

    // Show directories first
    for (var i = 0; i < dirs.length; i++) {
        var dir = dirs[i];
        console.log(indent + "📁 " + dir.Name + "/");
        if (depth < 2) { // Limit recursion depth
            showDirectoryStructure(dir.Path, depth + 1);
        }
    }

    // Show files
    for (var i = 0; i < files.length; i++) {
        var file = files[i];
        console.log(indent + "📄 " + file.Name + " (" + file.Size + " bytes)");
    }
}

// Execute main function
main(); 