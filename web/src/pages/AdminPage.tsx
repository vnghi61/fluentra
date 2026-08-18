import React, { useState } from "react";
import { Flag, Shield, Users } from "lucide-react";
import { AdminUserList, AdminFeatureFlags } from "@/features/admin";

type AdminTab = "users" | "flags";

export function AdminPage(): React.JSX.Element {
  const [activeTab, setActiveTab] = useState<AdminTab>("users");

  const tabs = [
    { key: "users" as AdminTab, label: "Learner Management", icon: Users },
    { key: "flags" as AdminTab, label: "Feature Flags", icon: Flag },
  ];

  return (
    <div className="max-w-6xl mx-auto space-y-8 py-4">
      {/* Header */}
      <header className="space-y-1">
        <div className="flex items-center gap-2">
          <span className="inline-flex items-center gap-1 rounded-md bg-indigo-500/10 px-2 py-0.5 text-xs font-semibold text-indigo-400 border border-indigo-500/20 uppercase tracking-wider">
            <Shield className="h-3 w-3" />
            Administration
          </span>
        </div>
        <h1 className="text-2xl font-bold text-slate-100">
          Platform Administration
        </h1>
        <p className="text-sm text-slate-400">
          Manage platform learners, enforce moderation, and configure system feature flags.
        </p>
      </header>

      {/* Tabs */}
      <div className="flex border-b border-slate-800 gap-2 pb-px overflow-x-auto">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = activeTab === tab.key;
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => setActiveTab(tab.key)}
              className={`flex items-center gap-2 whitespace-nowrap px-4 py-3 text-sm font-medium border-b-2 transition-colors min-h-[44px] cursor-pointer ${
                isActive
                  ? "border-indigo-500 text-indigo-400 bg-indigo-500/5 rounded-t-lg"
                  : "border-transparent text-slate-400 hover:text-slate-200 hover:border-slate-700"
              }`}
            >
              <Icon className="h-4 w-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Tab Panels */}
      <div>
        {activeTab === "users" && <AdminUserList />}
        {activeTab === "flags" && <AdminFeatureFlags />}
      </div>
    </div>
  );
}
