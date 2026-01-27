import js from '@eslint/js';

export default [
    // Global ignores
    {
        ignores: [
            'node_modules/**',
            'dist/**',
            '*.min.js',
            'package-lock.json',
            'config.json',
            'coverage/**'
        ]
    },
    // Recommended config
    js.configs.recommended,
    // Project-specific config
    {
        languageOptions: {
            ecmaVersion: 'latest',
            sourceType: 'module',
            globals: {
                // Browser globals
                window: 'readonly',
                document: 'readonly',
                navigator: 'readonly',
                console: 'readonly',
                setTimeout: 'readonly',
                clearTimeout: 'readonly',
                setInterval: 'readonly',
                clearInterval: 'readonly',
                fetch: 'readonly',
                localStorage: 'readonly',
                sessionStorage: 'readonly',
                URLSearchParams: 'readonly',
                URL: 'readonly',
                crypto: 'readonly',
                atob: 'readonly',
                btoa: 'readonly',
                confirm: 'readonly',
                alert: 'readonly',
                AbortSignal: 'readonly',
                AbortController: 'readonly',
                EventSource: 'readonly',
                DOMException: 'readonly',
                // Node.js globals
                process: 'readonly',
                Buffer: 'readonly',
                __dirname: 'readonly',
                __filename: 'readonly',
                global: 'readonly',
                module: 'readonly',
                require: 'readonly',
                exports: 'readonly'
            }
        },
        rules: {
            'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
            'no-console': 'warn',
            'prefer-const': 'error',
            'no-var': 'error',
            'eqeqeq': ['error', 'always'],
            'curly': ['error', 'all'],
            'brace-style': ['error', '1tbs'],
            'semi': ['error', 'always'],
            'quotes': ['error', 'single', { avoidEscape: true }],
            'comma-dangle': ['error', 'never'],
            'indent': ['error', 4, { SwitchCase: 1 }],
            'max-len': ['warn', { code: 200, ignoreUrls: true, ignoreStrings: true }]
        }
    },
    // Allow console statements in scripts directory (utility scripts need console output)
    {
        files: ['scripts/**/*.js'],
        rules: {
            'no-console': 'off'
        }
    },
    // Allow console statements in logger utility (it wraps console calls)
    {
        files: ['js/logger.js'],
        rules: {
            'no-console': 'off'
        }
    }
];
