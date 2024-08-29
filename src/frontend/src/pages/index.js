import Image from "next/image";
import { Inter } from "next/font/google";

import { MainDashboard } from "@/components/main-dashboard";

const inter = Inter({ subsets: ["latin"] });

export async function getServerSideProps() {
  // Fetch data from external API
  const res = await fetch('http://localhost:6555/apis')
  const repo = await res.json()
  // console.log(repo)
  // Pass data to the page via props
  return { props: { repo } }
}
 

export default function Home({repo}) {
  return (
    <main>
      <MainDashboard data={repo}/>
    </main>
  );
}
