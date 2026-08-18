import React from "react";
import { Link } from "@tanstack/react-router";
import { LogIn, LogOut, UserPlus } from "lucide-react";

export interface AppShellProps {
  children: React.ReactNode;
  user?: { role: string } | null | undefined;
  status?: "idle" | "authenticated" | "unauthenticated" | undefined;
  onLogout?: (() => void) | undefined;
}

export const AppShell: React.FC<AppShellProps> = ({
  children,
  user,
  status = "idle",
  onLogout,
}) => {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col md:flex-row">
      {/* Desktop Sidebar Navigation */}
      <aside className="hidden md:flex flex-col w-64 border-r border-slate-800 p-4 space-y-6 justify-between">
        <div className="space-y-6">
          <Link to="/" className="text-xl font-bold text-indigo-400 block">
            Fluentra
          </Link>
          <nav className="flex flex-col space-y-2">
            <Link
              to="/"
              className="flex items-center h-11 px-4 rounded-lg text-slate-300 hover:bg-slate-800 min-h-[44px] transition-colors"
            >
              Dashboard
            </Link>
            <Link
              to="/practice"
              className="flex items-center h-11 px-4 rounded-lg text-slate-300 hover:bg-slate-800 min-h-[44px] transition-colors"
            >
              Practice
            </Link>
            {status === "authenticated" && (
              <Link
                to="/settings"
                className="flex items-center h-11 px-4 rounded-lg text-slate-300 hover:bg-slate-800 min-h-[44px] transition-colors"
              >
                Settings
              </Link>
            )}
          </nav>
        </div>

        {/* Auth footer in sidebar */}
        <div className="border-t border-slate-800 pt-4 space-y-2">
          {status === "authenticated" && user ? (
            <div className="space-y-3">
              <div className="text-xs text-slate-400 px-2 truncate">
                Role:{" "}
                <span className="font-semibold text-slate-200 uppercase">
                  {user.role}
                </span>
              </div>
              <button
                type="button"
                onClick={onLogout}
                className="flex items-center gap-2 w-full h-11 px-4 rounded-lg text-rose-400 hover:bg-rose-500/10 min-h-[44px] transition-colors text-sm font-medium cursor-pointer"
              >
                <LogOut className="h-4 w-4" />
                Sign out
              </button>
            </div>
          ) : (
            <div className="flex flex-col space-y-2">
              <Link
                to="/login"
                className="flex items-center gap-2 h-11 px-4 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white min-h-[44px] transition-colors text-sm font-medium justify-center"
              >
                <LogIn className="h-4 w-4" />
                Sign in
              </Link>
              <Link
                to="/register"
                className="flex items-center gap-2 h-11 px-4 rounded-lg border border-slate-700 hover:bg-slate-800 text-slate-300 min-h-[44px] transition-colors text-sm font-medium justify-center"
              >
                <UserPlus className="h-4 w-4" />
                Create account
              </Link>
            </div>
          )}
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 p-4 pb-20 md:pb-4 max-w-7xl mx-auto w-full">
        {children}
      </main>

      {/* Mobile Bottom Navigation Bar */}
      <nav className="md:hidden fixed bottom-0 left-0 right-0 h-16 bg-slate-900 border-t border-slate-800 flex items-center justify-around z-50 px-2 pb-[env(safe-area-inset-bottom)]">
        <Link
          to="/"
          className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-indigo-400 text-xs font-medium"
        >
          Home
        </Link>
        <Link
          to="/practice"
          className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-slate-400 text-xs font-medium"
        >
          Practice
        </Link>
        {status === "authenticated" ? (
          <>
            <Link
              to="/settings"
              className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-slate-400 text-xs font-medium"
            >
              Settings
            </Link>
            <button
              type="button"
              onClick={onLogout}
              className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-rose-400 text-xs font-medium"
            >
              Logout
            </button>
          </>
        ) : (
          <Link
            to="/login"
            className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-slate-400 text-xs font-medium"
          >
            Sign in
          </Link>
        )}
      </nav>
    </div>
  );
};
