//!amo

// File System API Demo - Using fs.xxx syntax
// Demonstrates the improved filesystem API with IDE autocompletion support

function main() {
    console.log("📁 File System API Demo (fs.xxx syntax)");
    console.log("========================================");

    // Get current working directory using fs API
    var cwdResult = fs.cwd();
    if (cwdResult.error) {
        console.error("❌ Failed to get working directory:", cwdResult.error);
        return false;
    }
    
    console.log("📂 Current directory:", cwdResult.path);
    console.log("");

    // Create demo directory using fs.mkdir
    var demoDir = "./fs_api_demo";
    console.log("📁 Creating demo directory:", demoDir);
    
    var makeDirResult = fs.mkdir(demoDir);
    if (makeDirResult.error) {
        console.error("❌ Failed to create directory:", makeDirResult.error);
        return false;
    }
    console.log("✅ Directory created successfully");

    // Create demo files using fs.write
    console.log("");
    console.log("📄 Creating demo files with fs.write...");
    
    var files = [
        { name: "config.json", content: '{"name": "fs-demo", "version": "1.0.0"}' },
        { name: "README.md", content: "# FS API Demo\n\nThis demo shows the new fs.xxx API." },
        { name: "notes.txt", content: "Notes:\n- fs.write is easier than writeFile\n- fs.read is simpler" }
    ];

    for (var i = 0; i < files.length; i++) {
        var file = files[i];
        var filePath = fs.join([demoDir, file.name]);
        
        var writeResult = fs.write(filePath, file.content);
        if (writeResult.error) {
            console.error("❌ Failed to create file " + file.name + ":", writeResult.error);
        } else {
            console.log("✅ Created:", file.name, "(using fs.write)");
        }
    }

    // List directory contents using fs.readdir
    console.log("");
    console.log("📂 Directory contents (using fs.readdir):");
    
    var listResult = fs.readdir(demoDir);
    if (!listResult.success) {
        console.error("❌ Failed to list directory:", listResult.error);
    } else {
        for (var i = 0; i < listResult.files.length; i++) {
            var file = listResult.files[i];
            var icon = file.is_dir ? "📁" : "📄";
            var ext = fs.ext(file.name);
            console.log("  " + icon + " " + file.name + " (" + file.size + " bytes)" + (ext ? " [" + ext + "]" : ""));
        }
    }

    // Read file using fs.read
    console.log("");
    console.log("📖 Reading file (using fs.read):");
    
    var notesPath = fs.join([demoDir, "notes.txt"]);
    var readResult = fs.read(notesPath);
    if (readResult.error) {
        console.error("❌ Failed to read file:", readResult.error);
    } else {
        console.log("📄 Content of notes.txt:");
        console.log(readResult.content);
    }

    // File existence checking using fs.exists and fs.isFile
    console.log("");
    console.log("🔍 File existence checks:");
    console.log("fs.exists('" + notesPath + "'):", fs.exists(notesPath));
    console.log("fs.isFile('" + notesPath + "'):", fs.isFile(notesPath));
    console.log("fs.isDir('" + demoDir + "'):", fs.isDir(demoDir));

    // Get file info using fs.stat
    console.log("");
    console.log("📊 File info (using fs.stat):");
    var fileInfoResult = fs.stat(notesPath);
    if (!fileInfoResult.success) {
        console.error("❌ Failed to get file info:", fileInfoResult.error);
    } else {
        var fileInfo = fileInfoResult.data;
        console.log("  Name:", fileInfo.name);
        console.log("  Size:", fileInfo.size, "bytes");
        console.log("  Modified:", fileInfo.mod_time);
        console.log("  Is directory:", fileInfo.is_dir);
    }

    // Copy file using fs.copy
    console.log("");
    console.log("📋 Copying file (using fs.copy):");
    
    var srcFile = fs.join([demoDir, "notes.txt"]);
    var dstFile = fs.join([demoDir, "notes_backup.txt"]);
    
    var copyResult = fs.copy(srcFile, dstFile);
    if (copyResult.error) {
        console.error("❌ Failed to copy file:", copyResult.error);
    } else {
        console.log("✅ Copied notes.txt to notes_backup.txt");
    }

    // Append to file using fs.append
    console.log("");
    console.log("📝 Appending to file (using fs.append):");
    
    var appendResult = fs.append(dstFile, "\n- fs.append adds content to files");
    if (appendResult.error) {
        console.error("❌ Failed to append to file:", appendResult.error);
    } else {
        console.log("✅ Appended content to notes_backup.txt");
    }

    // Path operations using fs utilities
    console.log("");
    console.log("🔗 Path operations (using fs.xxx):");
    
    var testPath = "/home/user/documents/report.pdf";
    console.log("Original path:", testPath);
    console.log("fs.dirname():", fs.dirname(testPath));
    console.log("fs.basename():", fs.basename(testPath));
    console.log("fs.ext():", fs.ext(testPath));
    
    var splitResult = fs.split(testPath);
    console.log("fs.split() - dir:", splitResult.dir, "file:", splitResult.file);

    // Find files using fs.find
    console.log("");
    console.log("🔍 Finding files (using fs.find):");
    
    var findResult = fs.find(demoDir, "*.txt");
    if (findResult.error) {
        console.error("❌ Failed to find files:", findResult.error);
    } else {
        console.log("Found", findResult.files.length, "text files:");
        for (var i = 0; i < findResult.files.length; i++) {
            console.log("  📄", fs.basename(findResult.files[i]));
        }
    }

    // Get file size using fs.size
    console.log("");
    console.log("📏 File sizes (using fs.size):");
    
    var configPath = fs.join([demoDir, "config.json"]);
    var sizeResult = fs.size(configPath);
    if (sizeResult.error) {
        console.error("❌ Failed to get file size:", sizeResult.error);
    } else {
        console.log("config.json size:", sizeResult.size, "bytes");
    }

    // Directory size
    var dirSizeResult = fs.size(demoDir);
    if (dirSizeResult.error) {
        console.error("❌ Failed to get directory size:", dirSizeResult.error);
    } else {
        console.log("Total directory size:", dirSizeResult.size, "bytes");
    }

    // Demonstrate MD5 hash calculation
    console.log("");
    console.log("🔐 MD5 Hash (using fs.md5):");
    
    var jsonPath = fs.join([demoDir, "config.json"]);
    var md5Result = fs.md5(jsonPath);
    if (md5Result.error) {
        console.error("❌ Failed to calculate MD5 hash:", md5Result.error);
    } else {
        console.log("config.json MD5:", md5Result.hash);
    }
    
    var txtPath = fs.join([demoDir, "notes.txt"]);
    var md5Result2 = fs.md5(txtPath);
    if (md5Result2.error) {
        console.error("❌ Failed to calculate MD5 hash:", md5Result2.error);
    } else {
        console.log("notes.txt MD5:", md5Result2.hash);
    }

    // Cleanup
    var cleanup = getVar("cleanup") === "true";
    if (cleanup) {
        console.log("");
        console.log("🗑️ Cleaning up (using fs.rm)...");
        
        var deleteResult = fs.rm(demoDir);
        if (deleteResult.error) {
            console.error("❌ Failed to delete demo directory:", deleteResult.error);
        } else {
            console.log("✅ Demo directory cleaned up");
        }
    } else {
        console.log("");
        console.log("ℹ️  Demo files preserved in:", demoDir);
        console.log("   Use --var cleanup=true to auto-cleanup");
    }

    console.log("");
    console.log("🎉 File system API demo completed!");
    console.log("");
    console.log("💡 Benefits of fs.xxx API:");
    console.log("   • Shorter, cleaner syntax");
    console.log("   • IDE autocompletion support");
    console.log("   • Multiple aliases (fs.read vs fs.readFile)");
    console.log("   • Unified interface (file system + path operations)");
    console.log("   • More intuitive naming");
    
    return true;
}

// Execute main function
main(); 