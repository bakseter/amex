import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import { defineConfig, globalIgnores } from 'eslint/config';
import globals from 'globals';
import tseslint from 'typescript-eslint';
import eslintPluginUnicorn from 'eslint-plugin-unicorn';
import simpleImportSort from 'eslint-plugin-simple-import-sort';

export default defineConfig([
    globalIgnores(['dist']),
    {
        files: ['**/*.{ts,tsx}'],
        extends: [
            js.configs.all,
            tseslint.configs.strictTypeChecked,
            {
                languageOptions: {
                    parserOptions: {
                        projectService: true,
                    },
                },
            },
            tseslint.configs.stylisticTypeChecked,
            reactHooks.configs.flat.recommended,
            reactRefresh.configs.vite,
        ],
        plugins: {
            unicorn: eslintPluginUnicorn.configs.recommended,
            'simple-import-sort': simpleImportSort,
        },
        languageOptions: {
            ecmaVersion: 2020,
            globals: globals.browser,
        },
        rules: {
            'sort-keys': 'off',
            'one-var': 'off',
            'no-ternary': 'off',
            '@typescript-eslint/no-invalid-void-type': 'off', // used by RTK Query
            'max-lines-per-function': 'off',
            'no-warning-comments': 'off',
            complexity: 'off',
            'no-console': 'warn',
            'class-methods-use-this': 'warn', // TODO: don't use classes and enable this again
            'no-magic-numbers': [
                'error',
                {
                    ignore: [0, 1, 100],
                    ignoreArrayIndexes: true,
                    ignoreDefaultValues: true,
                    ignoreEnums: true,
                },
            ],
            'unicorn/no-null': 'off',
            'sort-imports': 'off',
            'simple-import-sort/imports': 'error',
            'simple-import-sort/exports': 'error',
        },
    },
]);
