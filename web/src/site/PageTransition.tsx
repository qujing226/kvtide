import { motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

type PageTransitionProps = {
  children: ReactNode;
};

export function PageTransition({ children }: PageTransitionProps) {
  const prefersReducedMotion = useReducedMotion();
  const animationProps = prefersReducedMotion
    ? {}
    : {
        initial: { opacity: 0, y: 8 },
        animate: { opacity: 1, y: 0 },
        exit: { opacity: 0, y: -6 },
        transition: { duration: 0.2, ease: [0.22, 1, 0.36, 1] as const },
      };

  return (
    <motion.div className="page-transition" {...animationProps}>
      {children}
    </motion.div>
  );
}
