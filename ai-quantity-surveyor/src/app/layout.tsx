import type { Metadata } from "next";
import { connection } from "next/server";
import { Geist, Geist_Mono } from "next/font/google";
import AuthProvider from "./auth-provider";
import ThemeProvider from "./theme-provider";
import { PlanProvider } from "./plans";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "AI Quantity Surveyor",
  description: "Upload a residential floor plan, confirm AI-extracted dimensions, preview a 3D layout, and export plaster and paint estimates.",
};

export default async function RootLayout({ children }: LayoutProps<"/">) {
  await connection();
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        <ThemeProvider>
          <AuthProvider required={process.env.NODE_ENV === "production" || process.env.APP_ENV === "production"} supabaseUrl={(process.env.SUPABASE_URL ?? "").trim()} supabaseAnonKey={(process.env.SUPABASE_ANON_KEY ?? "").trim()}>
            <PlanProvider>{children}</PlanProvider>
          </AuthProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
