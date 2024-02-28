/// <reference types="vite/client" />
interface ImportMetaEnv {
    readonly DIMO_API_BASEURL: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}