import { Navbar } from "@/components/navbar";
import { Hero } from "@/components/hero";
import { ImageCarousel } from "@/components/image-carousel";
// import { FeaturesFloat } from "@/components/features-float";
import { Ecosystem } from "@/components/ecosystem";
import { AiLayer } from "@/components/ai-layer";
// import { Architecture } from "@/components/architecture";
import { HowItWorks } from "@/components/how-it-works";

export default function Home() {
  return (
    <main className="min-h-screen bg-background text-foreground overflow-hidden">
      <Navbar />
      <div className="min-h-[100vh] flex flex-col pt-[100px] justify-between">
        <div className="flex-1 flex items-center justify-center">
            <Hero />
        </div>
        <div className="mb-4">
            <ImageCarousel />
        </div>
      </div>
      <Ecosystem />
      <AiLayer />
      <HowItWorks />
      {/* <Architecture /> */}
      {/* <FeaturesFloat /> */}
    </main>
  );
}
