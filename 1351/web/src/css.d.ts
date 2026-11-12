// esbuild bundles imported CSS into static/app.css; this keeps tsc from rejecting the import.
declare module "*.css";
