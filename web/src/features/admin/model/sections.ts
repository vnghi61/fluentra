import {
  AlertTriangle,
  BookA,
  BookOpen,
  ClipboardCheck,
  DollarSign,
  FileCheck,
  Flag,
  Gauge,
  ListChecks,
  Users,
  type LucideIcon,
} from "lucide-react";

import { PERMISSIONS, usePermissions } from "./permissions";

/**
 * The administrative sections, in one place because two things now render them:
 * the sidebar group in the app frame and the tab row AdminPage keeps for narrow
 * screens, where the sidebar is not drawn. Two lists would drift, and the one
 * that drifts is the one nobody is looking at.
 *
 * `path` is the section's address. These are real routes rather than component
 * state, which is what lets the sidebar link to them at all — and what makes a
 * reload, a bookmark and the back button land where the reader left off.
 */

export type AdminSectionKey =
  | "users"
  | "content"
  | "review"
  | "questions"
  | "reports"
  | "vocabulary"
  | "flags"
  | "ai"
  | "moderation"
  | "payouts";

export interface AdminSection {
  key: AdminSectionKey;
  /** The route this section lives at, e.g. "/admin/users". */
  path: string;
  labelKey: string;
  labelFallback: string;
  Icon: LucideIcon;
  /**
   * Any one of these is enough to see the section.
   *
   * `admin` is one role but its permissions are granted individually, so an
   * administrator without `system.flags` must not be offered a Feature Flags
   * section whose every action answers 403. The server re-checks regardless —
   * see each operation's `x-permission` in api/openapi/openapi.yaml.
   */
  anyOf: readonly string[];
}

export const ADMIN_SECTIONS: readonly AdminSection[] = [
  {
    key: "users",
    path: "/admin/users",
    labelKey: "page.learnerManagement",
    labelFallback: "Learners",
    Icon: Users,
    anyOf: [PERMISSIONS.userList],
  },
  {
    key: "content",
    path: "/admin/content",
    labelKey: "page.contentLibrary",
    labelFallback: "Content",
    Icon: BookOpen,
    anyOf: [
      PERMISSIONS.contentEdit,
      PERMISSIONS.contentReview,
      PERMISSIONS.contentPublish,
    ],
  },
  {
    key: "review",
    path: "/admin/review",
    labelKey: "page.reviewQueue",
    labelFallback: "Review Queue",
    Icon: ClipboardCheck,
    anyOf: [PERMISSIONS.contentReview, PERMISSIONS.contentPublish],
  },
  {
    key: "questions",
    path: "/admin/questions",
    labelKey: "page.questionBank",
    labelFallback: "Question Bank",
    Icon: ListChecks,
    anyOf: [PERMISSIONS.questionbankRead, PERMISSIONS.questionbankCreate],
  },
  {
    key: "reports",
    path: "/admin/reports",
    labelKey: "adminReports.tabLabel",
    labelFallback: "Reported Items",
    Icon: AlertTriangle,
    anyOf: [PERMISSIONS.contentReview, PERMISSIONS.contentEdit],
  },
  {
    key: "vocabulary",
    path: "/admin/vocabulary",
    labelKey: "page.vocabulary",
    labelFallback: "Vocabulary",
    Icon: BookA,
    anyOf: [PERMISSIONS.contentEdit, PERMISSIONS.contentCreate],
  },
  {
    key: "flags",
    path: "/admin/flags",
    labelKey: "page.featureFlags",
    labelFallback: "Feature Flags",
    Icon: Flag,
    anyOf: [PERMISSIONS.systemFlags],
  },
  {
    key: "ai",
    path: "/admin/ai",
    labelKey: "page.aiUsage",
    labelFallback: "AI Usage",
    Icon: Gauge,
    anyOf: [PERMISSIONS.adminDashboard],
  },
  {
    key: "moderation",
    path: "/admin/moderation",
    labelKey: "adminModeration.tabLabel",
    labelFallback: "Course Moderation",
    Icon: FileCheck,
    anyOf: [
      PERMISSIONS.moderationRead,
      PERMISSIONS.moderationAct,
      PERMISSIONS.contentReview,
      PERMISSIONS.contentPublish,
    ],
  },
  {
    key: "payouts",
    path: "/admin/payouts",
    labelKey: "adminPayouts.tabLabel",
    labelFallback: "Creator Payouts",
    Icon: DollarSign,
    anyOf: [PERMISSIONS.adminDashboard],
  },
] as const;

export interface VisibleAdminSections {
  sections: AdminSection[];
  isLoading: boolean;
}

/** The sections this administrator may actually open. */
export function useVisibleAdminSections(): VisibleAdminSections {
  const { can, isLoading } = usePermissions();
  return {
    // Nothing is offered on a guess: until the read lands, `can` answers no to
    // everything and the list is empty rather than optimistic.
    sections: ADMIN_SECTIONS.filter((section) => section.anyOf.some(can)),
    isLoading,
  };
}

/** The section a path belongs to, or undefined for an unknown one. */
export function sectionForPath(pathname: string): AdminSection | undefined {
  return ADMIN_SECTIONS.find((section) => section.path === pathname);
}
