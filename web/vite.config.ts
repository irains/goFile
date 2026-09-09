import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: '../assets/web',
    emptyOutDir: true,
    manifest: 'manifest.json',
    sourcemap: false,
    rollupOptions: {
      input: 'index.html',
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined;
          if (id.includes('react-ace')) return 'editor';
          if (id.includes('ace-builds')) return undefined;
          if (id.includes('@mui') || id.includes('@emotion')) return 'mui';
          if (id.includes('react-router')) return 'router';
          if (id.includes('@tanstack/react-query')) return 'query';
          // React and React DOM both depend on loose-envify. Keeping that small
          // shared dependency with React avoids a vendor -> react -> vendor
          // circular chunk relationship.
          if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/scheduler/') || id.includes('/loose-envify/')) return 'react';
          return 'vendor';
        }
      }
    }
  }
});
