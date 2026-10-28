import { Detail, DetailHeading } from "../components/fb";

export function DocsPage() {
  return (
    <Detail k="docs">
      <DetailHeading title="Docs" subtitle="Documentation" />
      <p className="max-w-xl text-muted-foreground">Notes for this installation will be listed on this page.</p>
    </Detail>
  );
}
