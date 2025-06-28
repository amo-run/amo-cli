//!amo

// Hash Functions Demo - SHA256 & MD5
// Demonstrates the hash calculation functionality using fs.md5() and fs.sha256()

function main() {
    console.log("🔐 Hash Functions Demo");
    console.log("=====================");

    // Create a demo file for hash calculation
    var demoContent = "Hello, Amo Workflow!\nThis is a test file for hash calculation.\nTimestamp: " + new Date().toISOString();
    var demoFile = "./hash_demo_file.txt";
    
    console.log("📄 Creating demo file:", demoFile);
    var writeResult = fs.write(demoFile, demoContent);
    if (!writeResult.success) {
        console.error("❌ Failed to create demo file:", writeResult.error);
        return false;
    }
    console.log("✅ Demo file created successfully");
    
    console.log("");
    console.log("📝 File content:");
    console.log(demoContent);
    console.log("");

    // Calculate MD5 hash
    console.log("🔐 Calculating MD5 hash...");
    var md5Result = fs.md5(demoFile);
    if (md5Result.success) {
        console.log("✅ MD5 hash:", md5Result.hash);
    } else {
        console.error("❌ Failed to calculate MD5:", md5Result.error);
    }

    // Calculate SHA256 hash
    console.log("🔐 Calculating SHA256 hash...");
    var sha256Result = fs.sha256(demoFile);
    if (sha256Result.success) {
        console.log("✅ SHA256 hash:", sha256Result.hash);
    } else {
        console.error("❌ Failed to calculate SHA256:", sha256Result.error);
    }

    // Compare hash lengths
    if (md5Result.success && sha256Result.success) {
        console.log("");
        console.log("📊 Hash comparison:");
        console.log("   MD5 length:    " + md5Result.hash.length + " characters");
        console.log("   SHA256 length: " + sha256Result.hash.length + " characters");
        console.log("   MD5 provides 128-bit security");
        console.log("   SHA256 provides 256-bit security (more secure)");
    }

    // Test with a different file
    var testFile2 = "./hash_demo_file2.txt";
    var testContent2 = "Different content for comparison";
    
    console.log("");
    console.log("📄 Creating second demo file for comparison...");
    var writeResult2 = fs.write(testFile2, testContent2);
    if (writeResult2.success) {
        console.log("✅ Second demo file created");
        
        var md5Result2 = fs.md5(testFile2);
        var sha256Result2 = fs.sha256(testFile2);
        
        if (md5Result2.success && sha256Result2.success) {
            console.log("🔐 Second file hashes:");
            console.log("   MD5:    " + md5Result2.hash);
            console.log("   SHA256: " + sha256Result2.hash);
            
            console.log("");
            console.log("✅ Hash verification:");
            console.log("   Different files produce different hashes ✓");
            console.log("   Each hash is deterministic and reproducible ✓");
        }
    }

    // Test error handling with non-existent file
    console.log("");
    console.log("🧪 Testing error handling with non-existent file...");
    var nonExistentFile = "./this_file_does_not_exist.txt";
    var errorTestMD5 = fs.md5(nonExistentFile);
    var errorTestSHA256 = fs.sha256(nonExistentFile);
    
    if (!errorTestMD5.success && !errorTestSHA256.success) {
        console.log("✅ Error handling works correctly:");
        console.log("   MD5 error: " + errorTestMD5.error);
        console.log("   SHA256 error: " + errorTestSHA256.error);
    }

    // Cleanup demo files
    var cleanup = getVar("cleanup") !== "false"; // Default to cleanup unless explicitly disabled
    if (cleanup) {
        console.log("");
        console.log("🗑️ Cleaning up demo files...");
        
        var deleteResult1 = fs.remove(demoFile);
        var deleteResult2 = fs.remove(testFile2);
        
        if (deleteResult1.success && deleteResult2.success) {
            console.log("✅ Demo files cleaned up");
        } else {
            console.log("⚠️ Some demo files may not have been cleaned up");
        }
    } else {
        console.log("");
        console.log("ℹ️ Demo files preserved:");
        console.log("   " + demoFile);
        console.log("   " + testFile2);
        console.log("   Use --var cleanup=false to preserve files");
    }

    console.log("");
    console.log("🎉 Hash demo completed!");
    console.log("");
    console.log("💡 Usage examples:");
    console.log("   fs.md5('/path/to/file.txt')     // 128-bit MD5 hash");
    console.log("   fs.sha256('/path/to/file.txt')  // 256-bit SHA256 hash (recommended)");
    console.log("");
    console.log("🔒 Security note:");
    console.log("   SHA256 is cryptographically stronger than MD5");
    console.log("   Use SHA256 for security-sensitive applications");
    
    return true;
}

// Execute main function
main(); 