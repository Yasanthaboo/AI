"use client";

import { createContext, Fragment, useContext, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import type { Session, SupabaseClient, User } from "@supabase/supabase-js";
import { clearDrawings } from "./drawing-store";
import { getBrowserSupabaseClient } from "./auth-client";

type AuthState = { enabled: boolean; user: User | null; signOut: () => Promise<void> };
type AuthMode = "sign-in" | "sign-up" | "reset" | "update-password";

const AuthContext = createContext<AuthState>({ enabled: false, user: null, signOut: async () => {} });

export function useAuth() {
  return useContext(AuthContext);
}

function clearLocalAccountState() {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem("quantity-surveyor-session");
  void clearDrawings().catch(() => {});
}

function AuthPanel({ client, initialMode, onRecoveryComplete }: {
  client: SupabaseClient;
  initialMode: AuthMode;
  onRecoveryComplete?: () => void;
}) {
  const [mode, setMode] = useState<AuthMode>(initialMode);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    setError("");
    try {
      if (mode === "sign-up") {
        if (password.length < 8) throw new Error("Use a password with at least 8 characters.");
        if (password !== confirmation) throw new Error("Passwords do not match.");
        const { data, error: authError } = await client.auth.signUp({
          email: email.trim(),
          password,
          options: { emailRedirectTo: window.location.origin },
        });
        if (authError) throw new Error("Unable to create the account. Check the details or try signing in.");
        if (!data.session) setMessage("Check your email for a verification link, then return here to sign in.");
        return;
      }
      if (mode === "sign-in") {
        const { error: authError } = await client.auth.signInWithPassword({ email: email.trim(), password });
        if (authError) throw new Error("Email or password is incorrect, or the account still needs verification.");
        return;
      }
      if (mode === "reset") {
        const { error: authError } = await client.auth.resetPasswordForEmail(email.trim(), { redirectTo: window.location.origin });
        if (authError) throw authError;
        setMessage("If an account exists for that email, a password reset link is on its way.");
        return;
      }
      if (password.length < 8) throw new Error("Use a password with at least 8 characters.");
      if (password !== confirmation) throw new Error("Passwords do not match.");
      const { error: authError } = await client.auth.updateUser({ password });
      if (authError) throw authError;
      setPassword("");
      setConfirmation("");
      setMessage("Password updated.");
      onRecoveryComplete?.();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Authentication could not be completed.");
    } finally {
      setBusy(false);
    }
  };

  const signInWithGoogle = async () => {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const { error: authError } = await client.auth.signInWithOAuth({ provider: "google", options: { redirectTo: window.location.origin } });
      if (authError) throw new Error(authError.message);
    } catch {
      setError("Google sign-in could not be started. Try again or use email.");
      setBusy(false);
    }
  };

  const title = mode === "sign-up" ? "Create your account" : mode === "reset" ? "Reset your password" : mode === "update-password" ? "Choose a new password" : "Sign in";

  return <main className="auth-screen">
    <section className="auth-panel" aria-labelledby="auth-title">
      <header className="auth-heading">
        <span className="brand__mark" aria-hidden="true">QS</span>
        <p className="eyebrow">AI Quantity Surveyor</p>
        <h1 id="auth-title">{title}</h1>
        <p className="muted">Your projects and drawings stay in your account.</p>
      </header>
      {(mode === "sign-in" || mode === "sign-up") && <div className="auth-tabs" role="tablist" aria-label="Account access">
        <button type="button" role="tab" aria-selected={mode === "sign-in"} className={mode === "sign-in" ? "auth-tab auth-tab--active" : "auth-tab"} onClick={() => { setMode("sign-in"); setError(""); setMessage(""); }}>Sign in</button>
        <button type="button" role="tab" aria-selected={mode === "sign-up"} className={mode === "sign-up" ? "auth-tab auth-tab--active" : "auth-tab"} onClick={() => { setMode("sign-up"); setError(""); setMessage(""); }}>Create account</button>
      </div>}
      {(mode === "sign-in" || mode === "sign-up") && <button type="button" className="button button--ghost auth-google" disabled={busy} onClick={() => void signInWithGoogle()}>
        Continue with Google
      </button>}
      {(mode === "sign-in" || mode === "sign-up") && <div className="auth-divider"><span>or use email</span></div>}
      <form className="auth-form" onSubmit={submit}>
        {mode !== "update-password" && <label className="field"><span>Email</span><input type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label>}
        {mode !== "reset" && <label className="field"><span>{mode === "update-password" ? "New password" : "Password"}</span><input type="password" autoComplete={mode === "sign-up" || mode === "update-password" ? "new-password" : "current-password"} minLength={8} required value={password} onChange={(event) => setPassword(event.target.value)} /></label>}
        {(mode === "sign-up" || mode === "update-password") && <label className="field"><span>Confirm password</span><input type="password" autoComplete="new-password" minLength={8} required value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>}
        {error && <p className="alert alert--error" role="alert">{error}</p>}
        {message && <p className="alert alert--info" role="status">{message}</p>}
        <button type="submit" className="button button--primary auth-submit" disabled={busy}>
          {busy ? <><span className="spinner" aria-hidden="true" /> Working…</> : mode === "sign-up" ? "Create account" : mode === "reset" ? "Send reset link" : mode === "update-password" ? "Update password" : "Sign in"}
        </button>
      </form>
      {mode === "sign-in" && <button type="button" className="link-button auth-link" onClick={() => { setMode("reset"); setMessage(""); setError(""); }}>Forgot password?</button>}
      {(mode === "reset" || mode === "update-password") && <button type="button" className="link-button auth-link" onClick={() => { setMode("sign-in"); setMessage(""); setError(""); }}>Back to sign in</button>}
      <p className="auth-footnote">By continuing, you agree to use project drawings only with services you have approved.</p>
    </section>
  </main>;
}

export default function AuthProvider({ children, supabaseUrl, supabaseAnonKey, required = false }: {
  children: ReactNode;
  supabaseUrl: string;
  supabaseAnonKey: string;
  required?: boolean;
}) {
  const hasAnyConfig = Boolean(supabaseUrl || supabaseAnonKey);
  const hasCompleteConfig = Boolean(supabaseUrl && supabaseAnonKey);
  const [client] = useState(() => hasCompleteConfig ? getBrowserSupabaseClient(supabaseUrl, supabaseAnonKey) : null);
  const [session, setSession] = useState<Session | null>(null);
  const [ready, setReady] = useState(!hasCompleteConfig);
  const [passwordRecovery, setPasswordRecovery] = useState(false);
  const [loadError, setLoadError] = useState("");
  const currentUserID = useRef<string | null>(null);

  useEffect(() => {
    if (!client) return;
    let mounted = true;
    let authChanges = 0;
    const { data: { subscription } } = client.auth.onAuthStateChange((event, nextSession) => {
      authChanges += 1;
      const previousUserID = currentUserID.current;
      const nextUserID = nextSession?.user.id ?? null;
      if (event === "SIGNED_OUT" || (previousUserID && nextUserID && previousUserID !== nextUserID)) clearLocalAccountState();
      currentUserID.current = nextUserID;
      setSession(nextSession);
      if (event === "PASSWORD_RECOVERY") setPasswordRecovery(true);
      setReady(true);
    });
    const initialChangeCount = authChanges;
    void client.auth.getSession().then(({ data, error }) => {
      if (!mounted || authChanges !== initialChangeCount) return;
      if (error) setLoadError("Could not restore your sign-in session. Sign in again.");
      currentUserID.current = data.session?.user.id ?? null;
      setSession(data.session);
      setReady(true);
    });
    return () => {
      mounted = false;
      subscription.unsubscribe();
    };
  }, [client]);

  if ((required || hasAnyConfig) && !hasCompleteConfig) return <main className="auth-screen"><section className="auth-panel"><h1>Sign-in is not configured</h1><p className="alert alert--error">Set both SUPABASE_URL and SUPABASE_ANON_KEY in the web service environment, not only on the API service.</p></section></main>;
  if (!client) return <AuthContext.Provider value={{ enabled: false, user: null, signOut: async () => {} }}>{children}</AuthContext.Provider>;
  if (!ready) return <main className="auth-screen"><p className="loading-card" role="status"><span className="spinner" aria-hidden="true" /> Checking your session…</p></main>;
  if (loadError) return <main className="auth-screen"><section className="auth-panel"><h1>Session unavailable</h1><p className="alert alert--error" role="alert">{loadError}</p><button type="button" className="button button--primary auth-submit" onClick={() => void client.auth.signOut()}>Return to sign in</button></section></main>;
  if (passwordRecovery) return <AuthPanel key="recovery" client={client} initialMode="update-password" onRecoveryComplete={() => setPasswordRecovery(false)} />;
  if (!session) return <AuthPanel key="access" client={client} initialMode="sign-in" />;

  const signOut = async () => {
    const { error } = await client.auth.signOut();
    if (error) throw error;
  };
  return <AuthContext.Provider value={{ enabled: true, user: session.user, signOut }}><Fragment key={session.user.id}>{children}</Fragment></AuthContext.Provider>;
}