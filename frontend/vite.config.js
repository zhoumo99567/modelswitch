import { defineConfig } from 'vite';

export default defineConfig({
 optimizeDeps: { esbuildOptions: { target: 'es2020' } },
 server: { port: 5103, strictPort: true },
 // pi-agent-core's schema validation uses BigInt; Safari 14 is the minimum.
 build: { target: ['es2020', 'chrome87', 'safari14'] },
});
