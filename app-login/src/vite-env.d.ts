/// <reference types="vite/client" />
interface ImportMetaEnv {
    readonly DIMO_API_BASEURL: string;
    readonly DIMO_PROJECT_ID: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}