import { Detail, DetailHeading } from "../components/fb";

export function DocsPage() {
  return (
    <Detail k="docs">
      <DetailHeading title="Docs" mono={false} />
      <p className="max-w-xl text-muted-foreground">
        Guides for this environment will appear here. Until then, use the project README.
      </p>
    </Detail>
  );
}
