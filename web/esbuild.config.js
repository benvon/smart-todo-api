import { build } from 'esbuild';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const isWatch = process.argv.includes('--watch');

// Entry points for different pages
const entryPoints = {
    'app': join(__dirname, 'js', 'app-entry.js'),
    'index': join(__dirname, 'js', 'index-entry.js')
};

// Build configuration
const buildOptions = {
    entryPoints: entryPoints,
    bundle: true,
    outdir: join(__dirname, 'dist'),
    format: 'iife',
    target: 'es2020',
    sourcemap: true,
    minify: !isWatch,
    splitting: false,
    // Keep function names for better debugging
    keepNames: true,
    // Global name for each entry point (based on filename)
    entryNames: '[name]',
    // Platform for browser
    platform: 'browser',
    // Prefer CommonJS for better compatibility (chrono-node has issues with ESM default export)
    mainFields: ['main', 'module', 'browser'],
    // Resolve extensions
    resolveExtensions: ['.tsx', '.ts', '.jsx', '.js', '.css', '.json']
};

if (isWatch) {
    buildOptions.watch = {
        onRebuild(error, _result) {
            if (error) {
                console.error('Build failed:', error);
            } else {
                console.log('Build succeeded');
            }
        }
    };
}

build(buildOptions)
    .then(() => {
        if (!isWatch) {
            console.log('Build complete');
        } else {
            console.log('Watching for changes...');
        }
    })
    .catch((error) => {
        console.error('Build error:', error);
        process.exit(1);
    });
