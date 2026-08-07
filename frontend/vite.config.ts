/// <reference types="vitest" />
import {defineConfig} from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
    // Vitest already transforms TSX through Vite's esbuild pipeline. Keeping the
    // React refresh plugin active in jsdom makes the legacy Wails template expect
    // a browser HMR preamble that is deliberately absent in tests.
    plugins: process.env.VITEST ? [] : [react()],
    test: {
        environment: 'jsdom',
        setupFiles: './src/test/setup.ts',
        css: true,
    },
});
