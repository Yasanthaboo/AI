import { createClient, type SupabaseClient } from "@supabase/supabase-js";

let client: SupabaseClient | null = null;
let configuredEndpoint = "";
let configuredKey = "";

export function getBrowserSupabaseClient(endpoint: string, anonKey: string): SupabaseClient | null {
  if (!endpoint || !anonKey) return null;
  if (!client || endpoint !== configuredEndpoint || anonKey !== configuredKey) {
    client = createClient(endpoint, anonKey, {
      auth: { autoRefreshToken: true, detectSessionInUrl: true, persistSession: true },
    });
    configuredEndpoint = endpoint;
    configuredKey = anonKey;
  }
  return client;
}

export async function currentAccessToken(): Promise<string | null> {
  if (!client) return null;
  const { data, error } = await client.auth.getSession();
  if (error) throw error;
  return data.session?.access_token ?? null;
}