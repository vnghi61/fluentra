import React from "react";

interface AppShellProps {
  children: React.ReactNode;
}

export const AppShell: React.FC<AppShellProps> = ({ children }) => {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col md:flex-row">
      {/* Desktop Sidebar Navigation */}
      <aside className="hidden md:flex flex-col w-64 border-r border-slate-800 p-4 space-y-6">
        <div className="text-xl font-bold text-indigo-400">Fluentra</div>
        <nav className="flex flex-col space-y-2">
          <a
            href="/"
            className="flex items-center h-11 px-4 rounded-lg bg-indigo-600 text-white font-medium min-h-[44px]"
          >
            Dashboard
          </a>
          <a
            href="/courses"
            className="flex items-center h-11 px-4 rounded-lg text-slate-300 hover:bg-slate-800 min-h-[44px]"
          >
            Courses
          </a>
          <a
            href="/review"
            className="flex items-center h-11 px-4 rounded-lg text-slate-300 hover:bg-slate-800 min-h-[44px]"
          >
            Review (SRS)
          </a>
          <a
            href="/settings"
            className="flex items-center h-11 px-4 rounded-lg text-slate-300 hover:bg-slate-800 min-h-[44px]"
          >
            Settings
          </a>
        </nav>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 p-4 pb-20 md:pb-4 max-w-7xl mx-auto w-full">
        {children}
      </main>

      {/* Mobile Bottom Navigation Bar */}
      <nav className="md:hidden fixed bottom-0 left-0 right-0 h-16 bg-slate-900 border-t border-slate-800 flex items-center justify-around z-50 px-2 pb-[env(safe-area-inset-bottom)]">
        <a
          href="/"
          className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-indigo-400 text-xs font-medium"
        >
          Home
        </a>
        <a
          href="/courses"
          className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-slate-400 text-xs font-medium"
        >
          Learn
        </a>
        <a
          href="/review"
          className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-slate-400 text-xs font-medium"
        >
          Review
        </a>
        <a
          href="/settings"
          className="flex flex-col items-center justify-center min-w-[44px] min-h-[44px] text-slate-400 text-xs font-medium"
        >
          Profile
        </a>
      </nav>
    </div>
  );
};
