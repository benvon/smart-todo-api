#!/usr/bin/env node
/**
 * Verify that certain dependencies are not imported in source code.
 * This helps identify unused dependencies that may have security vulnerabilities
 * but don't actually affect the application runtime.
 */

import { readFileSync, readdirSync, statSync } from 'fs';
import { join, extname } from 'path';
import { fileURLToPath } from 'url';
import { dirname } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Dependencies to check for (packages that should not be imported)
const UNUSED_DEPENDENCIES = [
    '@actions/http-client',
    '@actions/core'
];

// File extensions to check
const JS_EXTENSIONS = ['.js', '.jsx', '.ts', '.tsx', '.mjs'];

// Directories to exclude
const EXCLUDE_DIRS = ['node_modules', 'dist', 'coverage', '.git'];

// Files to exclude (scripts that check for dependencies)
const EXCLUDE_FILES = ['verify-unused-deps.js'];

/**
 * Recursively find all JavaScript files in a directory
 */
function findJSFiles(dir, fileList = []) {
    const files = readdirSync(dir);

    for (const file of files) {
        const filePath = join(dir, file);
        const stat = statSync(filePath);

        if (stat.isDirectory()) {
            // Skip excluded directories
            if (!EXCLUDE_DIRS.includes(file)) {
                findJSFiles(filePath, fileList);
            }
        } else if (stat.isFile()) {
            const ext = extname(file);
            if (JS_EXTENSIONS.includes(ext) && !EXCLUDE_FILES.includes(file)) {
                fileList.push(filePath);
            }
        }
    }

    return fileList;
}

/**
 * Check if a file imports any of the unused dependencies
 */
function checkFileForImports(filePath) {
    const content = readFileSync(filePath, 'utf-8');
    const found = [];

    for (const dep of UNUSED_DEPENDENCIES) {
        // Check for various import patterns:
        // - import ... from '@actions/http-client'
        // - require('@actions/http-client')
        // - from '@actions/http-client'
        const patterns = [
            new RegExp(`import\\s+.*\\s+from\\s+['"]${dep.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}['"]`, 'g'),
            new RegExp(`require\\(['"]${dep.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}['"]\\)`, 'g'),
            new RegExp(`from\\s+['"]${dep.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}['"]`, 'g')
        ];

        for (const pattern of patterns) {
            if (pattern.test(content)) {
                found.push({ dependency: dep, file: filePath });
                break; // Only report once per dependency per file
            }
        }
    }

    return found;
}

/**
 * Main function
 */
function main() {
    const projectRoot = join(__dirname, '..');
    const jsFiles = findJSFiles(projectRoot);
    const violations = [];

    console.log(`Checking ${jsFiles.length} JavaScript files for unused dependencies...\n`);

    for (const file of jsFiles) {
        const found = checkFileForImports(file);
        if (found.length > 0) {
            violations.push(...found);
        }
    }

    if (violations.length > 0) {
        console.error('❌ Found imports of unused dependencies:\n');
        for (const violation of violations) {
            console.error(`  - ${violation.dependency} imported in ${violation.file}`);
        }
        console.error('\nThese dependencies should be removed from package.json if not needed.');
        process.exit(1);
    } else {
        console.log('✅ No imports found for unused dependencies.');
        console.log(`   Verified that ${UNUSED_DEPENDENCIES.join(', ')} are not imported in source code.`);
        process.exit(0);
    }
}

main();
