// `/` and an unknown path both redirect here, including when authentication is off.
export const overviewPath = "/overview";
export const docsPath = "/docs";
export const servicesPath = "/services";
export const backtestsPath = "/backtests";
export const strategiesPath = "/strategies";

export const homePath = overviewPath;

export const shellNav = [
  { to: overviewPath, label: "Overview" },
  { to: docsPath, label: "Docs" },
  { to: servicesPath, label: "Services" },
  { to: backtestsPath, label: "Backtests" },
  { to: strategiesPath, label: "Strategies" },
] as const;
