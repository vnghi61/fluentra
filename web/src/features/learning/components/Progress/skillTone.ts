import {
  BookOpen,
  FileText,
  Headphones,
  Mic,
  PenLine,
  SpellCheck,
  type LucideIcon,
} from "lucide-react";

export const SKILL_ORDER = [
  "vocabulary",
  "grammar",
  "reading",
  "listening",
  "speaking",
  "writing",
] as const;

export type Skill = (typeof SKILL_ORDER)[number];

export interface SkillToneConfig {
  icon: LucideIcon;
  chip: string;
  bar: string;
}

export const SKILL_TONE: Record<Skill, SkillToneConfig> = {
  vocabulary: {
    icon: BookOpen,
    chip: "bg-blue-500/10 text-blue-700 dark:bg-blue-400/15 dark:text-blue-300",
    bar: "bg-blue-500 dark:bg-blue-400",
  },
  grammar: {
    icon: SpellCheck,
    chip: "bg-violet-500/10 text-violet-700 dark:bg-violet-400/15 dark:text-violet-300",
    bar: "bg-violet-500 dark:bg-violet-400",
  },
  reading: {
    icon: FileText,
    chip: "bg-emerald-500/10 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300",
    bar: "bg-emerald-500 dark:bg-emerald-400",
  },
  listening: {
    icon: Headphones,
    chip: "bg-rose-500/10 text-rose-700 dark:bg-rose-400/15 dark:text-rose-300",
    bar: "bg-rose-500 dark:bg-rose-400",
  },
  speaking: {
    icon: Mic,
    chip: "bg-orange-500/10 text-orange-700 dark:bg-orange-400/15 dark:text-orange-300",
    bar: "bg-orange-500 dark:bg-orange-400",
  },
  writing: {
    icon: PenLine,
    chip: "bg-cyan-500/10 text-cyan-700 dark:bg-cyan-400/15 dark:text-cyan-300",
    bar: "bg-cyan-500 dark:bg-cyan-400",
  },
};
