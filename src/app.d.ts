/// <reference types="@cloudflare/workers-types" />

declare global {
  namespace App {
    interface Platform {
      env?: {
        DB?: D1Database;
        ASSETS?: Fetcher;
        PUBLIC_MOCK_MODE?: string;
        PARAKEET_API_URL?: string;
        PARAKEET_API_KEY?: string;
        PARAKEET_MODEL?: string;
      };
    }
  }
}

export {};
