import { Link } from "react-router-dom";
import { Detail, DetailHeading } from "../components/fb";
import { homePath, shellNav } from "../shell";

export function OverviewPage() {
  const links = shellNav.filter((item) => item.to !== homePath);
  return (
    <Detail k="overview">
      <DetailHeading title="Overview" mono={false} />
      <p className="max-w-xl">
        Welcome. Use the sidebar or the links below — overview details come in a later update.
      </p>
      <ul className="mt-6 flex flex-col gap-2">
        {links.map((item) => (
          <li key={item.to}>
            <Link className="text-primary" to={item.to}>
              {item.label}
            </Link>
          </li>
        ))}
      </ul>
    </Detail>
  );
}
