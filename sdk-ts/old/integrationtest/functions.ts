import { getFunctions, logger } from "../src";

const main = async () => {
  const functions = await getFunctions({ localFunctions: [
    {
      name: "exa",
      description: "exa",
      url: "http://localhost:1400",
    },
  ]});
  functions.forEach((f) => {
    logger.info(f.name);
  });
};

main();
