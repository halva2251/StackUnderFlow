import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { AuthProvider } from "@/context/AuthContext";
import Link from "next/link";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "StackUnderFlow",
  description: "Confidently wrong answers to your tech questions",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        <AuthProvider>
          <Header />
          <main className="flex-1 max-w-4xl mx-auto w-full px-4 py-6">{children}</main>
        </AuthProvider>
      </body>
    </html>
  );
}

function Header() {
  return (
    <header className="border-b bg-white">
      <div className="max-w-4xl mx-auto px-4 py-3 flex items-center justify-between">
        <Link href="/" className="text-xl font-bold text-orange-600">
          StackUnderFlow
        </Link>
        <nav className="flex items-center gap-4 text-sm">
          <Link href="/" className="text-gray-600 hover:text-gray-900">
            Questions
          </Link>
          <Link
            href="/ask"
            className="bg-orange-500 text-white px-3 py-1.5 rounded-md hover:bg-orange-600 transition-colors"
          >
            Ask Question
          </Link>
          <Link href="/login" className="text-gray-600 hover:text-gray-900">
            Login
          </Link>
        </nav>
      </div>
    </header>
  );
}
