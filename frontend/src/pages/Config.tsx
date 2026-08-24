import About from "../components/Config/About"
import Basic from "../components/Config/Basic"
import Donate from "../components/Config/Donate"
import Influx from "../components/Config/Influx"
import Prometheus from "../components/Config/Prometheus"
import Retention from "../components/Config/Retention"
import Scan from "../components/Config/Scan"

function Config() {

  return (
    <div class="row config-page">
      <div class="col-md config-column">
        
        <Basic></Basic>
        
        <div class="mt-4">
          <Donate></Donate>
        </div>
        <div class="mt-4">
          <Scan></Scan>
        </div>
        <div class="mt-4 mb-4">
          <Retention></Retention>
        </div>
      </div>
      <div class="col-md config-column">
        
        <Influx></Influx>
        
        <div class="mt-4">
          <Prometheus></Prometheus>
        </div>
        <div class="mt-4">
          <About></About>
        </div>
      </div>
    </div>
  )
}

export default Config
