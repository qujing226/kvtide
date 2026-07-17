import { RoutePage } from "../site/RoutePage";
import { SchedulerLab } from "./SchedulerLab";

type LabPageProps = {
  focusOnMount: boolean;
};

export function LabPage({ focusOnMount }: LabPageProps) {
  return (
    <RoutePage title="Lab" focusOnMount={focusOnMount}>
      <SchedulerLab />
    </RoutePage>
  );
}
