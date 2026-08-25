import { ReactNode } from "react";
import { ActiveBottom, usePortalStore } from "../../store/portalStore";

export type BottomPanelShellProps = {
  activeKey: ActiveBottom;
  testId: string;
  children: ReactNode;
  asLog?: boolean;
  hidden?: boolean;
};

export function BottomPanelShell({ activeKey, testId, children, asLog, hidden: hiddenProp }: BottomPanelShellProps) {
  const activeBottom = usePortalStore((s) => s.activeBottom);
  const hidden = hiddenProp !== undefined ? hiddenProp : activeBottom !== activeKey;
  return (
    <div
      data-testid={testId}
      className="h-[280px] lg:h-[340px] overflow-y-auto flex flex-col"
      role={asLog ? "log" : undefined}
      aria-live={asLog ? "polite" : undefined}
      aria-relevant={asLog ? "additions" : undefined}
      aria-hidden={hidden ? "true" : undefined}
      hidden={hidden ? true : undefined}
      style={hidden ? { display: "none" } : undefined}
    >
      {children}
    </div>
  );
}

export default BottomPanelShell;
