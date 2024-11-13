/// <reference types="vite/client" />
interface ImportMetaEnv {
    readonly DIMO_API_BASEURL: string;
    readonly DIMO_PROJECT_ID: string;
    readonly DIMO_CLIENT_ID: string;
    readonly DIMO_REDIRECT_URI: string;
    readonly DIMO_ENVIRONMENT: string;
    readonly DIMO_PERMISSION_TEMPLATE_ID: string;
    readonly DIMO_MODE: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}